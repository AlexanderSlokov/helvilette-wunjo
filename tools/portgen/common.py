import json, re
from gen import ruby_literal_to_go, ruby_value_json, go_string

def lit(src): return ruby_literal_to_go(src)

def opt_str(kw, key):
    return lit(kw[key]) if key in kw else '""'

def log_options(kw):
    fields = []
    if kw.get("timestamps") != "false":
        fields.append("Timestamps: true")
    names = {"container_id": "ContainerID", "since": "Since", "lines": "Lines", "grep": "Grep", "grep_options": "GrepOptions"}
    for key, field in names.items():
        if key in kw:
            fields.append(field + ": " + lit(kw[key]))
    return "LogOptions{" + ", ".join(fields) + "}"

def env_arg(kw):
    return "c.env(t, " + go_string(ruby_value_json(kw.get("env", "{}"))) + ")"

def strings(pos):
    return "[]string{" + ", ".join(lit(p) for p in pos) + "}"

def none(go):
    return lambda pos, kw: "." + go + "()"
