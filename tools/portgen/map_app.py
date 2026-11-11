import re
from common import *

NEW_CASE = "newAppCase(t)"

def receiver(src):
    m = re.match(r"^new_command(\((.*?)\))?(\..*)?$", src)
    if not m:
        return None, None
    role, host, extra = '"web"', '"1.1.1.1"', ""
    if m.group(2):
        from gen import parse_call_args
        _, kw = parse_call_args(m.group(2))
        role = lit(kw.pop("role")) if "role" in kw else role
        host = lit(kw.pop("host")) if "host" in kw else host
        for key, value in kw.items():
            extra += ".extra(t, " + json.dumps(key) + ", " + go_string(ruby_value_json(value)) + ")"
    return "c" + extra + ".app(t, " + role + ", " + host + ")", m.group(3) or ""

NO_ARGS = ["start", "info", "list_images", "remove_images", "tag_latest_image", "live", "remove", "remove_containers",
           "list_containers", "list_container_names", "current_running_container_id", "current_running_version",
           "remove_proxy_app_directory", "extract_assets", "clean_up_assets"]

def camel(name):
    special = {"current_running_container_id": "CurrentRunningContainerID"}
    return special.get(name, "".join(p.capitalize() for p in name.split("_")))

METHODS = {name: none(camel(name)) for name in NO_ARGS}
METHODS.update({
    "run": lambda pos, kw: ".Run(" + opt_str(kw, "hostname") + ")",
    "stop": lambda pos, kw: ".Stop(" + opt_str(kw, "version") + ")",
    "deploy": lambda pos, kw: ".Deploy(" + lit(kw["target"]) + ")",
    "remove_container": lambda pos, kw: ".RemoveContainer(" + lit(kw["version"]) + ")",
    "container_id_for": lambda pos, kw: ".ContainerIDFor(" + lit(kw["container_name"]) + ", false)",
    "list_versions": lambda pos, kw: ".ListVersions(" + strings(pos) + ", " + (lit(kw["statuses"]) if "statuses" in kw else "nil") + ")",
    "logs": lambda pos, kw: ".Logs(" + log_options(kw) + ")",
    "follow_logs": lambda pos, kw: ".FollowLogs(" + lit(kw.pop("host")) + ", " + log_options(kw) + ")",
    "maintenance": lambda pos, kw: ".Maintenance(" + ("intValue(" + kw["drain_timeout"] + ")" if "drain_timeout" in kw else "nil") + ", " + opt_str(kw, "message") + ")",
    "execute_in_new_container": lambda pos, kw: ".ExecuteInNewContainer(" + strings(pos) + ", false, " + kw.get("detach", "false") + ", " + env_arg(kw) + ")",
    "execute_in_existing_container": lambda pos, kw: ".ExecuteInExistingContainer(" + strings(pos) + ", false, " + env_arg(kw) + ")",
    "execute_in_new_container_over_ssh": lambda pos, kw: ".ExecuteInNewContainerOverSSH(" + strings(pos) + ", " + env_arg(kw) + ")",
    "execute_in_existing_container_over_ssh": lambda pos, kw: ".ExecuteInExistingContainerOverSSH(" + strings(pos) + ", " + env_arg(kw) + ")",
    "run_over_ssh": lambda pos, kw: ".RunOverSSH(w(" + lit(pos[0]) + "), " + lit(kw["host"]) + ")",
    "sync_asset_volumes": lambda pos, kw: ".SyncAssetVolumes(" + (lit(kw["old_version"]) if "old_version" in kw else '""') + ")",
})
