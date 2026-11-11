import re
from common import *
from gen import parse_call_args

NEW_CASE = "newBuilderCase(t)"
PRE = [("#{remote_arch}", "arm64"), ("#{local_arch}", "amd64"), ("=> remote_arch", '=> "arm64"')]

def receiver(src):
    m = re.match(r"^new_builder_command(\((.*?)\))?(\.(?:target|push|name|create|validate_image|first_mirror|clone|clone_reset_steps).*)?$", src)
    if not m:
        return None, None
    extra = ""
    if m.group(2):
        args = m.group(2).strip()
        _, kw = parse_call_args(args)
        for key, value in kw.items():
            extra += ".deep(t, " + json.dumps(key) + ", " + go_string(ruby_value_json(value)) + ")"
    return "c" + extra + ".builder(t)", m.group(3) or ""

METHODS = {
    "target": lambda pos, kw: ".Target()",
    "name": lambda pos, kw: ".Name()",
    "build_options": lambda pos, kw: "!.BuildOptions()",
    "push": lambda pos, kw: "!.Target().Push(" + (lit(pos[0]) if pos else '"registry"') + ", false, " + kw.get("no_cache", "false") + ")",
    "create": lambda pos, kw: "!.Target().Create()",
    "validate_image": lambda pos, kw: ".ValidateImage()",
    "first_mirror": lambda pos, kw: ".FirstMirror()",
}
