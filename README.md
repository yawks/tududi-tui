# tududi-tui

A keyboard-first terminal client for [Tududi](https://github.com/chrisvel/tududi), built with Bubble Tea.

## Setup

Create an API key in Tududi, then copy `config.example.json` to your platform config directory:

- macOS: `~/Library/Application Support/tududi-tui/config.json`
- Linux: `~/.config/tududi-tui/config.json`

`base_url` is the Tududi server root, without `/api` (for example `http://localhost:3002`).

```sh
go run .
```

## Keys

| Key | Action |
| --- | --- |
| `j`/`k`, arrows | Move |
| `tab` | Switch sidebar/content |
| `enter` | Open sidebar item |
| `n` | New task (from any view) |
| `N` | New project |
| `T` | New tag |
| `e`, `d` | Edit, delete |
| `space` | Complete/reopen task |
| `u` | Undo the last completion while its toast is visible (6 seconds) |
| `h` | Toggle completed tasks |
| `v` | Month/week/working-week calendar |
| `r` | Refresh |
| `?` | Help |
| `q` | Quit |

In forms, use `tab` or the arrow keys to move, `ctrl+s` to save, and `enter` on the due-date field to open the calendar. Project and tag fields open keyboard selectors with `enter`.
Tag and project colors accept any value supported by Tududi (normally `#RRGGBB`); press `enter` on the Color field for the built-in palette or type a custom value directly.

In the calendar, use the arrows to select a day, `enter` to browse its tasks, `esc` to return to day navigation, and `[`/`]` to change period.

The active project, the last task filter, and the calendar view are restored from `state.json` in the same config directory. Use the explicit `All` entries to clear either filter.

The minimum supported terminal size is 80×20. The task detail panel is hidden automatically on narrower terminals.
