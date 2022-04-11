Quickstart
==========
The [quickstart](../quickstart.sh) script is meant to set up a quick-and-dirty
JEC2 environment which should work in most cases.  It does the following.

1. Make `$HOME/jec2`, JEServer's [work directory](./jeserver.md#work-directory)
2. Build [JEServer](./jeserver.md), put it in `$HOME/jec2/bin`,
   and start it going with its [defaults](./jeserver.md#defaults)
3. Build [JEGenImplant](./jegenimplant.md), put it in `$HOME/jec2/bin`, and use
   it to build several [implants](./jeimplant.md) in `$HOME/jec2/implants`
