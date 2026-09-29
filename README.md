# Core War demo

A small, self-contained Core War / Redcode visualiser. Two built-in warriors
fight in an endless best-of-three attract mode on a 320×240 display: teal on
the left and red on the right. The 8000-cell core occupies a 160×50 grid
below the status bar. Recently written or executed core cells fade through an
8-colour ramp.

The desktop program uses SDL3. The same simulator and UI can target the
Gopher Badge (RP2040 + ST7789) under TinyGo.

## Desktop

SDL3 development files must be installed. On macOS with Homebrew:

```sh
brew install sdl3
go run ./cmd/corewar
```

To pit your own warriors against each other, pass directories or files:

```sh
go run ./cmd/corewar ~/warriors hill/            # only these warriors
go run ./cmd/corewar -builtin ~/warriors mine.rc # these plus the built-ins
```

Directories are walked recursively for `.red` files; other files, and hidden
files and directories such as `.git`, are ignored. A file named directly is
loaded whatever its extension. Any `.red` file that is not valid ICWS'94
Redcode (a 42-school champion, say) is skipped with a warning. At least two
warriors are needed.

Controls: `N` or `Enter` starts a new pairing, `Left` / `Right` replace the
left or right warrior with a random one, `Space` or `P` pauses, `+` / `-` (or
`Up` / `Down`) change simulation speed, and `Esc` or `Q` quits.

The winner of each best-of-three match stays on, keeping its side, against a
random challenger; after a drawn match both warriors are replaced.

## Build targets

```sh
make test       # run VM, assembler, warriors, and UI tests
make generate   # regenerate Mars Opcode/Modifier String methods
make snapshot   # write 2x PNG snapshots (frame-00060.png etc.)
make run        # run the SDL3 desktop demo
make badge      # produce corewar.uf2 for a Gopher Badge
```

`cmd/snapshot` accepts `-seed`, `-frames`, `-speed`, `-scale`, `-o`, and
`-builtin`, plus the same warrior paths as `cmd/corewar`; it is useful in a non-graphical build environment.

## Gopher Badge

The Gopher Badge build target configures its ST7789 on SPI0 at 32 MHz,
rotated to a 320×240 landscape display. It uses no framebuffer: the UI's
`Display` interface emits `FillRect` calls. New writes and instruction
execution put their core cell on a fixed 8000-element decay list. The UI
redraws only these cells as their brightness advances, then swap-removes a
cell when it reaches its terminal dim colour. The status bar is likewise
redrawn only when a field changes.

Button mapping: **up** increases speed and **down** decreases it, **left**
and **right** replace the left or right warrior with a random one, **A**
starts a new pairing, and **B** pauses/resumes.

The configured MARS core is 64 KB (8000 packed instructions); per-cell
owner/brightness/list bookkeeping is about 40 KB, and the maximum two
process queues add 32 KB. This leaves comfortable space within the RP2040's
264 KB RAM without allocating a 75 KB screen buffer. The Badge lists the
embedded warriors at boot but reads and assembles only the two fighting,
when their match starts, so the roster can grow without using RAM.

## Redcode support

The assembler implements the ICWS'94 instruction set and all 8 addressing
modes and 7 modifiers, with labels, `EQU`, `ORG`, `END`, expressions, and
standard constants such as `CORESIZE`, `PSPACESIZE` and `VERSION`. It also
supports the pMARS extensions most published warriors rely on:

- P-space: `LDP` / `STP` with a 500-cell private P-space per warrior that
  persists across the rounds of a match, and whose cell 0 holds the last
  round's result. `PIN` is accepted, but P-space is never shared.
- `;assert` comments are ignored: a warrior written for another hill, say
  `;assert CORESIZE==8192`, runs in the 8000-cell core and fares as it may.
- CWS'86/'88-style sources, which separate operands with whitespace
  (`mov < src 0`), load as long as the file has no commas at all.
- Text before the first `;redcode` line is ignored, so e-mailed entries load
  as-is.
- `EQU` values that begin with an addressing mode, as in `slot EQU #313`.

`FOR` / `ROF` blocks are implemented but currently disabled
(`redcode.ForEnabled`), so warriors using them are rejected.

Built-in warrior files are kept on disk as [`warriors/*.red`](warriors) and
compiled into the binary through the exported `warriors.FS` (`embed.FS`).
Built-ins that fail to assemble are left out of the roster with a warning. A warrior is named by its `;name`
comment, or after its file if it has none. Anything that is not ICWS'94
Redcode, such as 42-school Corewar `.name`/`.comment` champions, is rejected.

`go test -bench . ./redcode ./warriors` benchmarks the assembler. It reports
allocations as well as time, since on the Badge the garbage from assembling
each new challenger shares the heap with the core.
