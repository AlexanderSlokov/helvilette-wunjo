import re
from common import *

NEW_CASE = "newProxyCase(t)"
PRE = [("@config[:servers].first", '"1.1.1.1"')]

def receiver(src):
    m = re.match(r"^new_command(\..*)?$", src)
    return ("c.proxy(t)", m.group(1) or "") if m else (None, None)

def camel(name):
    return "".join(p.capitalize() for p in name.split("_"))

METHODS = {n: none(camel(n)) for n in ["run", "start", "info", "version", "ensure_proxy_directory", "read_boot_options",
    "read_image", "read_image_version", "read_run_command", "reset_boot_options", "reset_image", "reset_image_version",
    "reset_run_command", "remove_container", "remove_image"]}
METHODS.update({
    "ensure_apps_config_directory": none("EnsureAppsConfigDirectory"),
    "stop": lambda pos, kw: ".Stop(\"\")",
    "logs": lambda pos, kw: ".Logs(" + log_options(kw) + ")",
    "follow_logs": lambda pos, kw: ".FollowLogs(" + lit(kw.pop("host")) + ", " + log_options(kw) + ")",
})
