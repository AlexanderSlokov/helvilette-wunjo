"""Generates Go tests from the JSON that extract.rb writes for a Kamal test file.

The port uses it to translate Kamal's Minitest files without copying each expected
string by hand. extract.rb runs in Ruby 3.4 with Prism, so expected strings are the
values Ruby computes:

    docker run --rm -v "$PWD/tools/portgen":/x -v "$PWD/temp/kamal":/k -w /k ruby:3.4-alpine \
        ruby /x/extract.rb test/commands/app_test.rb > app_test.json
    python3 tools/portgen/gen.py app_test.json App tools/portgen/map_app.py \
        internal/kamal/commands app

Usage: python3 gen.py <json> <prefix> <mapping module> <out dir> <base name>
It writes <base>_test.go, then <base>_part2_test.go and so on, each at most
MAX_LINES lines long, split between tests, as AGENTS.md limits files to 500 lines.
The mapping module defines `receiver(src)` and `METHODS`, which translate Ruby calls
into Go expressions. Anything it cannot translate becomes a failing test, so no
Ruby assertion goes missing silently.
"""
import json
import re
import sys
import importlib.util
import os

MAX_LINES = 490


def camel(name):
    words = re.split(r"[^0-9a-zA-Z]+", name)
    return "".join(w[:1].upper() + w[1:] for w in words if w)


def go_string(value):
    if "`" not in value and "\r" not in value:
        return "`" + value + "`"
    return json.dumps(value, ensure_ascii=False)


def split_args(src):
    """Splits Ruby call arguments at top-level commas."""
    args, depth, cur, quote = [], 0, "", None
    i = 0
    while i < len(src):
        ch = src[i]
        if quote:
            cur += ch
            if ch == "\\":
                cur += src[i + 1]
                i += 2
                continue
            if ch == quote:
                quote = None
        elif ch in "\"'":
            quote = ch
            cur += ch
        elif ch in "([{":
            depth += 1
            cur += ch
        elif ch in ")]}":
            depth -= 1
            cur += ch
        elif ch == "," and depth == 0:
            args.append(cur.strip())
            cur = ""
        else:
            cur += ch
        i += 1
    if cur.strip():
        args.append(cur.strip())
    return args


def parse_call_args(src):
    positional, keywords = [], {}
    for arg in split_args(src):
        m = re.match(r"^([a-z_]+):\s+(.*)$", arg, re.S)
        if m:
            keywords[m.group(1)] = m.group(2).strip()
        else:
            positional.append(arg)
    return positional, keywords


def ruby_literal_to_go(src):
    """Translates a simple Ruby literal into a Go literal."""
    src = src.strip()
    if re.match(r'^"(?:[^"\\]|\\.)*"$', src) and "#{" not in src:
        return json.dumps(json.loads(src), ensure_ascii=False)
    if re.match(r"^-?\d+$", src):
        return json.dumps(src)
    if re.match(r"^:[a-z_]+$", src):
        return json.dumps(src[1:])
    if src.startswith("[") and src.endswith("]"):
        items = [ruby_literal_to_go(x) for x in split_args(src[1:-1])]
        return "[]string{" + ", ".join(items) + "}"
    if src in ("true", "false"):
        return src
    raise ValueError("cannot translate literal " + src)


def ruby_hash_to_json(src):
    src = src.strip()
    if src == "{}":
        return "{}"
    body = src[1:-1]
    pairs = []
    for pair in split_args(body):
        k, v = pair.split("=>", 1)
        pairs.append(json.dumps(json.loads(k.strip())) + ": " + ruby_value_json(v.strip()))
    return "{" + ", ".join(pairs) + "}"


def ruby_value_json(src):
    if src.startswith("{"):
        return ruby_hash_to_json(src)
    if src.startswith("["):
        return "[" + ", ".join(ruby_value_json(x) for x in split_args(src[1:-1])) + "]"
    return src


def translate_actual(src, mapping):
    src = re.sub(r"\s+", " ", src.strip())
    for pattern, replacement in getattr(mapping, "PRE", []):
        src = src.replace(pattern, replacement)
    tty = None
    m = re.match(r"^stub_stdin_(tty|file) \{ (.*) \}$", src)
    if m:
        tty = m.group(1) == "tty"
        src = m.group(2)
    if src == "command":
        return "command", tty
    m = re.match(r"^([a-z_]+)(\..*)?$", src)
    if m and m.group(1) in getattr(mapping, "VARIABLES", set()):
        recv, rest = m.group(1), m.group(2) or ""
    else:
        recv, rest = mapping.receiver(src)
    if recv is None:
        raise ValueError("no receiver for " + src)
    expr = recv
    while rest:
        m = re.match(r"^\.([a-z_?]+)(\((.*?)\))?(?=\.|$)", rest)
        if not m:
            raise ValueError("cannot parse chain " + rest)
        method, args = m.group(1), m.group(3)
        rest = rest[m.end():]
        if method == "join" and args == '" "':
            expr += ".String()"
            continue
        if method not in mapping.METHODS:
            raise ValueError("unmapped method " + method)
        positional, keywords = parse_call_args(args or "")
        piece = mapping.METHODS[method](positional, keywords)
        if piece.startswith("!"):
            expr = "must(t)(" + expr + piece[1:] + ")"
        else:
            expr += piece
    return expr, tty


def config_path(src):
    """Returns (path, value src) for @config[:a]["b"] = value."""
    m = re.match(r'^@config((?:\[(?::[a-z_]+|"[^"]+")\])+)\s*=\s*(.*)$', src, re.S)
    if not m:
        return None
    keys = re.findall(r'\[(?::([a-z_]+)|"([^"]+)")\]', m.group(1))
    return [a or b for a, b in keys], m.group(2)


def pre(src, mapping):
    for pattern, replacement in getattr(mapping, "PRE", []):
        src = src.replace(pattern, replacement)
    return src


def emit_statement(s, mapping, out):
    src = pre(s["src"], mapping)
    call = s.get("call")
    if src.startswith("@destination ="):
        out.append("\tc.destination = " + json.dumps(json.loads(src.split("=", 1)[1].strip())))
        return
    path = config_path(src)
    if path:
        keys, value = path
        out.append("\tc.set(t, " + go_string(ruby_value_json(re.sub(r"\s+", " ", value))) + ", " + ", ".join(json.dumps(k) for k in keys) + ")")
        return
    m = re.match(r"^@config\.delete\(:([a-z_]+)\)$", src)
    if m:
        out.append('\tc.raw.Delete("' + m.group(1) + '")')
        return
    m = re.match(r"^([a-z_]+) = (.*)$", src, re.S)
    if m:
        expr, tty = translate_actual(m.group(2), mapping)
        if tty is not None:
            out.append("\tc.tty = " + ("true" if tty else "false"))
        out.append("\t" + m.group(1) + " := " + expr)
        mapping.VARIABLES = getattr(mapping, "VARIABLES", set()) | {m.group(1)}
        return
    if call in ("assert_equal", "assert_match", "assert_no_match"):
        emit_assert(s, mapping, out)
        return
    raise ValueError("untranslated statement: " + src[:120])


def emit_assert(s, mapping, out):
    call = s["call"]
    expected, actual = s["args"][0], s["args"][1]
    expr, tty = translate_actual(actual["src"], mapping)
    if tty is not None:
        out.append("\tc.tty = " + ("true" if tty else "false"))
    if call == "assert_equal":
        value = expected.get("value")
        if value is None and "interpolated" in expected:
            text = pre(expected["interpolated"], mapping)
            if "#{" not in text:
                value = json.loads(text)
        if isinstance(value, list):
            out.append("\tassertWords(t, []string{" + ", ".join(json.dumps(str(v), ensure_ascii=False) for v in value) + "}, " + expr + ")")
        elif isinstance(value, str):
            out.append("\tassertText(t, " + go_string(value) + ", " + text_of(expr) + ")")
        else:
            raise ValueError("unsupported expected " + expected["src"][:80])
        return
    regex = expected["src"]
    m = re.match(r"^%r\{(.*)\}$", regex, re.S) or re.match(r"^/(.*)/$", regex, re.S)
    if not m:
        raise ValueError("unsupported regex " + regex)
    fn = "assertMatch" if call == "assert_match" else "assertNoMatch"
    out.append("\t" + fn + "(t, " + go_string(m.group(1)) + ", " + text_of(expr) + ")")


def text_of(expr):
    return expr


def header(source):
    return ("package commands\n\n"
            "// Generated by tools/portgen from " + source + " at Kamal 2.12.0, then\n"
            "// edited where a test needed a hand port. Each test keeps the Ruby test name.\n\n"
            'import "testing"\n\n')


def test_block(test, name, mapping, source):
    """Returns the Go source of one test, or a failing test when it cannot translate."""
    body = []
    mapping.VARIABLES = set()
    try:
        for s in test["statements"]:
            emit_statement(s, mapping, body)
    except ValueError as err:
        body = ['\tt.Fatal("not ported: ' + str(err).replace('"', "'").replace("\n", " ")[:200] + '")']
    lines = ["// " + test["name"] + " (" + source + ":" + str(test["line"]) + ")",
             "func " + name + "(t *testing.T) {", "\tc := " + mapping.NEW_CASE] + body + ["}", ""]
    return "\n".join(lines) + "\n"


def split_parts(head, blocks, max_lines=MAX_LINES):
    """Groups test blocks into files of at most max_lines lines, keeping test order."""
    parts, current = [], head
    for block in blocks:
        if current != head and (current + block).count("\n") > max_lines:
            parts.append(current)
            current = head
        current += block
    return parts + [current]


def part_path(out_dir, base, index):
    suffix = "" if index == 0 else "_part" + str(index + 1)
    return os.path.join(out_dir, base + suffix + "_test.go")


def write_parts(out_dir, base, parts):
    for index, text in enumerate(parts):
        with open(part_path(out_dir, base, index), "w") as out:
            out.write(text.rstrip("\n") + "\n")


def main():
    data = json.load(open(sys.argv[1]))
    prefix, out_dir, base = sys.argv[2], sys.argv[4], sys.argv[5]
    spec = importlib.util.spec_from_file_location("mapping", sys.argv[3])
    mapping = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mapping)
    seen, blocks = set(), []
    for test in data["tests"]:
        name = "Test" + prefix + camel(test["name"])
        while name in seen:
            name += "Again"
        seen.add(name)
        blocks.append(test_block(test, name, mapping, data["file"]))
    write_parts(out_dir, base, split_parts(header(data["file"]), blocks))


if __name__ == "__main__":
    main()
