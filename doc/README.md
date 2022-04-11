Documentation
=============
This directory contains information about running JEC2.  Despite its somewhat
simple featureset, it can be a bit persnickety.

Pastables
---------
The below table contains some of the more useful commands for using JEC2.  They
assume JEServer was set up with [`quickstart.sh`](./quickstart.sh.md).

Description                   | Command
------------------------------|--------
Connect to a local server     | `ssh -i $HOME/id_25519_operator -J 127.0.0.1:10022 server`
Connect to the latest implant | `ssh -i $HOME/id_25519_operator -J 127.0.0.1:10022 latest`
