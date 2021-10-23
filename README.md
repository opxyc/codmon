# gomon

A (very) simple tool to automatically run commands upon file update.

### Installation
```
go install github.com/opxyc/gomon
```

### Usage:
```
gomon [FLAGS] 'command1 [&& command ...]'
  -stdin
        attach to stdin of executing commands
  -v    get verbose output (for debugging)
  -w string
        file extensions to watch for
        Ex: Use 'gomon -w go,c' to watch for .go and .c files
```

### gomon.json
By default, gomon tracks changes of all files. We can change that behaviour by adding `gomon.json`.
```json
{
    "watch" : ["go", "c", "sh"],
    "exclude" : {
        "files": ["*test*"],
        "dirs": ["foo", "goo/goo"]
    },
    "cmd" : "go build . && ./foo"
}
```
- `watch` - gomon will track all files with the extension mentioned in this list.
- `exclude.files` - Exclude files from watching for changes
- `exclude.dirs` - Exclude directories from watching for changes.
- `cmd` - Mention the command(s) to be run. Note: Priority is given to the flag. (If `-cmd` is given via flag, then "cmd" from .json file is neglected.)

Note: gomon is not a tool specific to any language or environment. It can be used in any cases where a change(Rename, Move, Create, Write/Update) in the directory/files need to trigger some action (like running a build command etc.)