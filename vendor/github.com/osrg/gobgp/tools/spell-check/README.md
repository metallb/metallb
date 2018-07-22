# What's this

This script is a spell checker for GoBGP's source codes.

## Requirements

- [scspell3k](https://pypi.python.org/pypi/scspell3k): Spell checker for
  source code written in Python.

  ```bash
  pip install scspell3k
  ```

## How to use

Just run `scspell.sh`.

```bash
bash tools/spell-check/scspell.sh
```

Example of output:

```bash
# Format:
# path/to/file.go: <messages>
xxx/xxx.go: 'mispeld' not found in dictionary (from token 'Mispeld')
```

## Adding new words to dictionary

If you want to add new words to the dictionary for this spell checker, please
insert words into `tools/spell-check/dictionary.txt` or
`tools/spell-check/ignore.txt`.
