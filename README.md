Just Enough C2
==============
An opinionated C2 server and implant which does just enough to be effective.

Meant primarily for small teams operating on small numbers of targets mostly
in Linux (cloud, devops, etc) environments without too much fear of detection.

Under the hood, it's all just SSH with extra steps.

For legal use only.

Features
--------
- Single server binary
- Single client binary
- All comms over SSH, eventually optionally TLS-wrapped
- End-to-end encryption between operator SSH client and implant
- Upload/download/pasteboard copy (using [iTerm2](https://iterm2.com) magic
- Shell command execution
- Subprocess execution
- Server-side logging
- Incomplete documentation
-

TODO
----
- Client-side TLS
- Client-side DNS compatible with
  [dnsproxycommand](https://github.com/magisterquis/dnsproxycommand)
- `-R` / `tcpip-forward` / `RemoteForward`
- Unincomplete documentation
- Easier build and setup
- Upload/Downloads which don't require [iTerm2](https://iterm2.com)
- Implant buildable as shared object file
