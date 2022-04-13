Documentation
=============
This directory contains information about running JEC2.  Despite its somewhat
simple featureset, it can be a bit persnickety.

These docs are very much a work in progress.

Pastables
---------
The below table contains some of the more useful commands for using JEC2.  They
assume JEServer was set up with [`quickstart.sh`](./quickstart.sh.md).

Description                   | Command
------------------------------|--------
Connect to a local server     | `ssh -i $HOME/jec2/id_ed25519_operator -J 127.0.0.1:10022 server`
Connect to the latest implant | `ssh -i $HOME/jec2/id_ed25519_operator -J 127.0.0.1:10022 latest`

SSH Config
----------
The following SSH config works nicely for the default JEServer setup, as made
by [`quickstart.sh`](./quickstart.sh.md).

```ssh-config
ControlMaster auto
ControlPath ~/.ssh/sock/%C.sock
ControlPersist yes
ServerAliveInterval 30

Host jeserver
        HostName 127.0.0.1
        Port 10022
        IdentityFile ~/jec2/id_ed25519_operator

Host jeimplant
        HostName latest
        ProxyJump jeserver
        IdentityFile ~/jec2/id_ed25519_operator
```

Don't forget to `mkdir -p ~/.ssh/sock`
