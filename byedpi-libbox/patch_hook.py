#!/usr/bin/env python3
"""Patch libbox CommandServer to route profile content through byedpi.

Anchors target sing-box v1.14.x experimental/libbox/command_server.go:
  - func (s *CommandServer) StartOrReloadService(configContent string, ...)
  - func (s *CommandServer) CloseService() error

Idempotent: exits 0 without changes if the hook is already present.
"""
import re
import sys

def main(path: str) -> int:
    with open(path, "r", encoding="utf-8") as f:
        src = f.read()

    if "byedpi.PrepareAndStart" in src:
        print("[patch] already applied")
        return 0
    if "package libbox" not in src or "StartOrReloadService" not in src:
        print("[patch] ERROR: unexpected command_server.go layout")
        return 1

    # 1. import
    src, n = re.subn(
        r"\nimport \(\n",
        '\nimport (\n\t"github.com/sagernet/sing-box/experimental/libbox/byedpi"\n',
        src,
        count=1,
    )
    if n != 1:
        print("[patch] ERROR: import anchor not found")
        return 1

    # 2. StartOrReloadService body: prepare config + ensure proxy running.
    anchor_a = "\terr := s.StartedService.StartOrReloadService(s.ctx, configContent, &daemon.OverrideOptions{"
    repl_a = (
        "\tconfigContent, byedpiErr := byedpi.PrepareAndStart(configContent)\n"
        "\tif byedpiErr != nil {\n"
        "\t\treturn byedpiErr\n"
        "\t}\n"
        "\terr := s.StartedService.StartOrReloadService(s.ctx, configContent, &daemon.OverrideOptions{"
    )
    if anchor_a not in src:
        print("[patch] ERROR: StartOrReloadService anchor not found")
        return 1
    src = src.replace(anchor_a, repl_a, 1)

    # 3. CloseService: stop proxy together with the service.
    anchor_b = "func (s *CommandServer) CloseService() error {\n\treturn s.StartedService.CloseService()"
    repl_b = (
        "func (s *CommandServer) CloseService() error {\n"
        "\tbyedpi.Stop()\n"
        "\treturn s.StartedService.CloseService()"
    )
    if anchor_b in src:
        src = src.replace(anchor_b, repl_b, 1)
    else:
        print("[patch] WARN: CloseService anchor not found; proxy stops on process exit instead")

    with open(path, "w", encoding="utf-8") as f:
        f.write(src)
    print("[patch] hooks installed")
    return 0

if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
