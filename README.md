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
- All comms over SSH, optionally TLS-wrapped
- End-to-end encryption between operator SSH client and implant
- Upload/download/pasteboard copy (optionally using [iTerm2](https://iterm2.com) magic)
- Shell command execution
- Subprocess execution
- Server-side logging
- Forward/Reverse TCP tunnels
- Somewhat
  [broken](https://github.com/golang/go/issues?q=is%3Aissue+is%3Aopen+x%2Fnet%2Fwebdav+)
  built-in WebDAV server
- Incomplete documentation
-

TODO
----
- Client-side DNS compatible with
  [dnsproxycommand](https://github.com/magisterquis/dnsproxycommand)
- Unincomplete documentation
- Easier build and setup
- Implant buildable as shared object file
- Actually test TLS comms
