# Optional controller prompt artwork

Goro does not bundle controller artwork. Prompt text remains available by
default. Users who want Kenney Input Prompts artwork can unpack the CC0 pack
into a directory and set:

```text
GORO_INPUT_PROMPTS_DIR=/path/to/input-prompts
```

The directory must contain these family folders:

```text
generic/
playstation/
steamdeck/
switch/
xbox/
```

The resolver uses the filenames selected by `glyphName` in `inputprompt.go`.
The official source is https://kenney.nl/assets/input-prompts.
