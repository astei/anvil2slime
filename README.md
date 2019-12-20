# anvil2slime

**This tool is experimental.**

This tool converts Anvil worlds to the Slime region format. It loads all the Anvil
regions into memory and saves it as a Slime world.

## Usage

### Basic usage

Assuming you have an Anvil world ready, run `anvil2slime WORLD`. A `WORLD.slime`
will be generated in the base directory the world is in. You can change where the
output goes by using the `-o` flag, i.e. `anvil2slime -o test.slime WORLD`.

### Full usage

```
NAME:
   anvil2slime - converts Anvil worlds to Slime and back

USAGE:
   anvil2slime [global options] command [command options] [arguments...]

VERSION:
   0.0.0

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --output FILE, -o FILE  writes the Slime region to the specified FILE
   --help, -h              show help (default: false)
   --version, -v           print the version (default: false)
```

## Details

`anvil2slime` loads an Anvil world (with each region loading concurrently)
and outputs a Slime region. This tool has been tested with a few worlds and
should provide a perfect mapping of your Anvil worlds to Slime.

Currently, this tool relies on a fork of [Tnze/go-mc's NBT library](https://github.com/Tnze/go-mc/tree/master/nbt)
as it does not implement functionality that is required for this tool to work.

The reason why you'd want to use this tool over the Hypixel-provided `slime-tools` are
many:

* `slime-tools` saves in Slime version 1 format and thus doesn't save entities.
  `anvil2slime` saves in Slime version 3 and does save entities.
* `slime-tools` doesn't work on anything other than 64-bit Windows without manually
  injecting native libraries into the JAR. `anvil2slime` uses only pure Go dependencies
  and is thus highly portable.

The one disadvantage is that `anvil2slime` tends to produce larger files, however
this is an artifact of the [compression library used](https://github.com/klauspost/compress/tree/master/zstd).