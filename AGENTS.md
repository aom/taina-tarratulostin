# AGENTS.md

This file is for AI agents (and a future me). The human-facing usage guide is `README.md` (in Finnish). This file explains how the tool was built, what has and has not been verified, which problems came up, and how each was solved.

## What this is

`tarra` is a Go tool (a single file, `main.go`, standard library only; `go.mod` contains just `module tarra` and `go 1.21`) that prints text and screw-head symbols on a DYMO LabelManager 280 from macOS. It writes a PDF by hand and sends it to CUPS with `lp`. The user is Mika (Finnish speaker, developer); the purpose is labelling boxes of screws. Conversation language with the user is Finnish; this file is English on his request.

User requirements from the conversation:
- Language: Ruby or Go, **not Python** (stated explicitly; also saved in his preferences).
- Label: 12 mm tape, label length follows the text, **at most 42 mm**, text starts at the **left edge** (not centered).
- Content: text plus symbols for common screw-head/fastener types.

## Layout of `main.go`

1. Helvetica / Helvetica-Bold width tables (`regW`, `boldW`, ASCII 32-126) and `accentBase` for accented letters. `init` asserts both tables have 95 entries.
2. `canvas` helpers emitting PDF drawing operators (Bezier circles, rectangles, rotated rectangles, polygons, stars).
3. `symbols` table: names, description, draw function. Each symbol is drawn into an `s`-sized square.
4. `parseLine` recognises `:name:` symbols; `|` splits lines.
5. `renderLabel` derives the font size from the available height, shrinks it to fit the length limit (`-max` / `-len`), lays out the lines and rotates the page.
6. `buildPDF` assembles the PDF by hand (catalog, pages, two Type1 fonts with WinAnsiEncoding, one content stream per page, xref).
7. `main` prints each label as its own `lp` job (the paper length is per job). `-n` writes one combined PDF and opens it in Preview.

Coordinates are in millimetres: the content stream starts with a `cm` scale to points (`ptPerMM = 72/25.4`) followed by the rotation matrix.

Symbols currently: `ph`/`risti`, `pz`/`pozi`, `slot`/`ura`, `hex`/`kuusio`, `torx`, `nut`/`mutteri`, `washer`/`levy`, `pan`/`kupu` (dome head, side view; the Finnish alias used to be `pyori` and was renamed to `kupu` at the user's request), `csk`/`uppo`.

## Printer facts

Verified from output on the user's machine:
- CUPS printer name: `DYMO_LabelManager_280` (listed by `lpstat -p`).
- The driver's `PageSize` values are **portrait** and in points: `w18h…` (6 mm), `w26h…` (9 mm), `w35h…` (12 mm), with heights such as 144, 252, 4000. `Custom.WIDTHxHEIGHT` is supported.
- Other driver options: `MediaType` (06mm/09mm/12mm), `DymoContinuousPaper` (0/1), `DymoLabelAlignment` (Center/Left/Right), `DymoHalftoning` (Default/ErrorDiffusion/NLL), 180 dpi.
- For 12 mm tape the page width is **35 pt** (about 12.347 mm), so the `-tape` default is `35/ptPerMM`.
- Working configuration: portrait page, `-rot 270`, `-o media=Custom.35xN`, `-o DymoContinuousPaper=1`, `-voffset 2.4`. The user confirmed the result as "perfect".

Not verified:
- A blank lead of about 8 mm at the start of each label. Suspected to be the printer's own feed margin, not confirmed.
- The effect of the `DymoHalftoning` alternatives on thin lines (suggested, never tested).
- The fallbacks `-rot 90`, `-media w35h144`, `-media none` and `DymoContinuousPaper=0` were never tried.
- Other tape widths (6, 9 mm).

## Problems and solutions (chronological)

1. **Printer not visible in Dymo Connect** (it showed "LabelWriter printer - Virtual", then a *System Extension Blocked* dialog). Cause: macOS blocked DYMO's system extension. The user did not want to reboot, so we first tried the *Allow* button in Privacy & Security and adding the printer manually. Eventually the printer appeared in macOS Printers & Scanners as *Idle*. After that DYMO's own app was no longer needed, because printing works through CUPS.

2. **Started with Python.** The user rejected it ("that Python again"). Switched to Go. Go was chosen over Ruby because it compiles to one binary, avoiding macOS's old system Ruby and gem installs.

3. **Go module fetching blocked in the sandbox.** `proxy.golang.org` bypassed the egress proxy (noProxy list). `GOPROXY=direct` could download the module from GitHub, but `sum.golang.org` returned 403. The checksum check was deliberately not bypassed (no `GONOSUMDB`), since that host is blocked by policy. **Solution:** write the PDF by hand with the standard library. Bonus: zero dependencies, and `go build` works offline on the user's Mac.

4. **First print was blank (white label).** Cause: the tool produced a landscape 42 x 12 mm page, but the driver's page sizes are portrait `w35hN`. The user's `lpoptions -p DYMO_LabelManager_280 -l` output revealed it. **Solution:** a portrait page 35 pt wide, content rotated with `-rot 270` (text starts at the page top, which prints first), media `Custom.35xN`, and `DymoContinuousPaper=1`.

5. **Content sat too high on the label** (about 1.3 mm above, about 7 mm below, although it is centred in the PDF). The offset was estimated from the user's photo and `-voffset` was added with a default of 2.4 mm. It landed right on the first try.

6. **Thin rings broke up** under halftoning (ErrorDiffusion at 180 dpi). Symbol strokes were made about 35 % thicker. The user was happy with the result.

7. **Centering was a wrong default.** The first version used a fixed 42 mm label with centred text. The user corrected this: the tape is continuous, text starts at the left, and 42 mm is only the maximum. Hence `-len` defaults to 0 (text length), `-max` to 42 and `-align` to `left`.

8. **File delivery slip.** One update shipped only `main.go`, and the user asked where `go.mod` was. When delivering files, send every required file or state by name which ones are unchanged. The final deliverable is a zip (`main.go`, `go.mod`, `README.md`, `AGENTS.md`) the user unpacks.

## How to continue

- **Do not test by printing blindly.** Tape is consumed. Generate the PDF (`-n`), render it (`pdftoppm -r 400 -png`) and look at the image. Rotate the image 90 degrees to read it.
- The sandbox has no `lp`/`lpstat` and no printer, so **the printing path cannot be tested there**. The user runs every print test. Ask for command output and photos (a photo of the label is enough to measure offsets).
- `mcp__remote-devices__device_bash` is a Linux VM, not the user's Mac shell, so it cannot see CUPS.
- Run `gofmt -w` and `go vet` before delivering.
- The width tables are Helvetica values written from memory, and accented letters reuse the base letter's width. If text length looks off, check those first.
- New symbol: add names, description and draw function to the `symbols` table, then check the result by rendering the PDF.
- When defaults change, update the options table in `README.md` as well.
