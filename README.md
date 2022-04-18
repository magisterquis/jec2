Just Enough C2
==============
An opinionated C2 server and implant which does Just Enough to be effective.

Meant primarily for small teams operating on small numbers of targets mostly
in Linux (Cloud, DevOps, etc) environments without too much fear of detection.

Under the hood, it's all just SSH with extra steps.

For legal use only.

Documentation
-------------
Docs live in the [`doc/`](./doc) directory.  They're a work in progress.

Features
--------
- Single binaries for client and server
- All comms end-to-end encrypted over SSH, optionally TLS-wrapped
- Upload/download/pasteboard copy (optionally using [iTerm2](https://iterm2.com) magic)
- Shell command execution
- Subprocess execution
- Server-side logging
- Forward/Reverse TCP tunnels
- Somewhat
  [broken](https://github.com/golang/go/issues?q=is%3Aissue+is%3Aopen+x%2Fnet%2Fwebdav+)
  built-in WebDAV server
- Easyish build and setup

Quickstart
----------
1. Have git and the [Go compiler](https://go.dev/doc/install)
2. Work out the server's extrnal address or something which points at port
   10222 on the server
2. Get the source:
   `git clone https://github.com/magisterquis/jec2.git`
3. Set everything up the easy way:
   `cd jec2 && ./quickstart.sh ssh://SERVERADDR:10022`
4. Optionally watch server logs:
   `tail -f $HOME/jec2/log`
4. Optionally tweak `$HOME/jec2/conf.json` and `pkill -HUP jeserver`
5. Run an implant from `$HOME/jec2/implants` on a target somewhere
6. List connected implants:
   `ssh -i $HOME/jec2/id_ed25519_operator -p 10022 127.0.0.1 list`
7. Use the newest implant:
   `ssh -i $HOME/jec2/id_ed25519_operator -J 127.0.0.1:10022 latest`

Please see the [quickstart docs](./doc/quickstart.sh.md) for more details.
