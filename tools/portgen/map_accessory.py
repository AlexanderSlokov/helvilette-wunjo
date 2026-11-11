import re
from common import *

NEW_CASE = "newAccessoryCase(t)"

def receiver(src):
    m = re.match(r"^new_command\(:([a-z]+)\)(\..*)?$", src)
    if not m:
        return None, None
    return "c.accessory(t, " + json.dumps(m.group(1)) + ")", m.group(2) or ""

METHODS = {n: none("".join(p.capitalize() for p in n.split("_"))) for n in
           ["start", "stop", "pull_image", "remove_container", "remove_image", "remove"]}
METHODS.update({
    "current_image_id": none("CurrentImageID"),
    "run": lambda pos, kw: "!.Run(\"\")",
    "info": lambda pos, kw: ".Info(false, false)",
    "remove_image_id": lambda pos, kw: ".RemoveImageID(" + lit(pos[0]) + ")",
    "execute_in_existing_container": lambda pos, kw: ".ExecuteInExistingContainer(" + strings(pos) + ", false)",
    "execute_in_new_container": lambda pos, kw: "!.ExecuteInNewContainer(" + strings(pos) + ", false)",
    "logs": lambda pos, kw: ".Logs(" + log_options(kw) + ")",
    "follow_logs": lambda pos, kw: ".FollowLogs(" + lit(kw.pop("host")) + ", " + log_options(kw) + ")",
    "deploy": lambda pos, kw: ".Deploy(" + lit(kw["target"]) + ")",
})
