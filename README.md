# masm2wasm

`masm2wasm` translates a broad MASM32/Irvine32 classroom subset into standalone WASI WebAssembly console programs.

GitHub releases ship a single user-facing binary, also named `masm2wasm`, which can both build and run projects.

It is built around a MASM-like interpreter runtime:

1. `internal\masm` parses MASM/Irvine source into IR.
2. `vm` provides the register file, memory model, flags, stack, Irvine-style shims, and file I/O.
3. `cmd\vmtemplate` is a generic WASI runtime module with an embedded payload slot.
4. `internal\generator` patches that template with the translated program IR and emits a runnable `.wasm`.
5. `cmd\wasmrun` remains as a thin developer wrapper around the same runtime path used by `masm2wasm run`.

This design means:

- the generated executables are WebAssembly
- the translator itself is also compilable to WebAssembly
- the translator can run as WebAssembly without spawning a host compiler

## Current compatibility summary

This project is now aimed at **real Irvine-style student console programs**, not just the bundled Easy-MASM samples.

### Implemented well

- Core MASM data/code parsing for console programs
- Procedures with `PROTO`, `PROC`, `USES`, `LOCAL`, named parameters/locals, user-defined `INVOKE`, and `ret n`
- High-level MASM conditionals and loops: `.IF`, `.ELSEIF`, `.ELSE`, `.ENDIF`, `.WHILE`, `.ENDW`, `.REPEAT`, `.UNTIL`
- Common compile-time expressions and directives: `OFFSET`, `ADDR`, `TYPE`, `LENGTHOF` / `LENGTH`, `SIZEOF` / `SIZE`, constants with `=` / `EQU`, practical text aliases with `TEXTEQU`, top-level compile-time `IF` / `ELSE` / `ENDIF`, and data-generating `REPT` / `REPEAT`, compile-time `WHILE`, `FOR`, and `FORC`
- `COMMENT` block handling plus common conditional-jump aliases such as `jng`, `jnl`, `jnge`, `jnle`, `jna`, `jnb`, and `jnbe`, along with MASM-style textual condition operators like `EQ`, `NE`, `LT`, `LE`, `GT`, `GE`, `AND`, `OR`, and `NOT`
- Classroom-style addressing like `[esi]`, `[ebp+8]`, `array[esi*4]`, `[ebx + esi*TYPE table]`, and symbol-plus-displacement forms
- Practical MASM data/type features including `LABEL`, data/struct `ALIGN`, anonymous adjacent data/aggregate declarations, `DB` / `DW` / `DD` / `DQ` aliases, user-defined `STRUCT` / `UNION`, nested aggregate fields, aggregate initializers, and typed field access such as `worker.Years` and `(Rectangle PTR [esi]).UpperLeft.Y`
- Interactive console programs using `ReadString`, `ReadInt`, `ReadDec`, `ReadHex`, `ReadChar`, and line-buffered `ReadKey`
- Irvine string routines, file I/O, and REP/string-instruction style algorithms
- `REAL4` / `REAL8` data, practical `REAL10` acceptance for load/store-style examples, a practical x87 stack model, and the floating-point chapter-12 style console examples
- Built-in Irvine macros plus a practical user-defined `MACRO` / `ENDM` subset

### Supported instructions

- `mov`, `lea`
- `add`, `sub`, `inc`, `dec`, `neg`
- `xor`, `and`, `or`, `not`, `test`
- `cmp`
- `mul`, `imul`, `div`, `idiv`, `cdq`
- `movzx`, `movsx`, `xchg`
- `shl`, `shr`, `sar`, `sal`
- `push`, `pop`
- `pushad`, `popad`, `pushfd`, `popfd`, `leave`
- `cld`, `std`
- `lodsb`, `lodsw`, `lodsd`
- `stosb`, `stosw`, `stosd`
- `movsb`, `movsw`, `movsd`
- `cmpsb`, `cmpsw`, `cmpsd`
- `scasb`, `scasw`, `scasd`
- `rep`, `repe` / `repz`, `repne` / `repnz` on supported string instructions
- `finit`, `fld`, `fld1`, `fldz`, `fild`
- `fst`, `fstp`, `fstcw`, `fstsw`, `fldcw`, `fnstsw`, `fist`
- `fadd`, `fsub`, `fsubr`, `fmul`, `fdiv`, `fdivr`
- `fabs`, `fchs`, `fsqrt`, `frndint`, `ftst`
- `fcomi`, `fcomp`
- `fclex`, `fwait`, `fincstp`
- `jmp`
- `je`, `jz`, `jne`, `jnz`
- `jl`, `jle`, `jg`, `jge`, `jng`, `jnl`, `jnge`, `jnle`
- `jb`, `jbe`, `ja`, `jae`, `jc`, `jnc`, `jnb`, `jna`, `jnbe`, `jnae`
- `js`, `jns`, `jo`, `jno`
- `jcxz`, `jecxz`, `loop`
- `call`, `ret`, `exit`
- `nop`

### Supported Irvine32 procedures

#### Console and formatting

- `WriteString`
- `Crlf`
- `WriteInt`
- `WriteDec`
- `WriteChar`
- `WriteHex`
- `WriteHexB`
- `WriteBin`
- `WriteBinB`
- `DumpRegs`
- `DumpMem`
- `Clrscr`
- `WaitMsg`
- `Gotoxy`
- `SetTextColor`
- `GetTextColor`
- `GetMaxXY`
- `Delay`
- `GetMseconds`
- `GetCommandTail`
- `ReadKey`
- `ReadKeyFlush`
- `IsDigit`
- `WriteStackFrame`
- `WriteStackFrameName`

#### Input

- `ReadInt`
- `ReadDec`
- `ReadHex`
- `ReadString`
- `ReadChar`
- `ReadFloat`
- `ParseInteger32`
- `ParseDecimal32`

#### String routines

- `StrLength`
- `Str_length`
- `Str_copy`
- `Str_compare`
- `Str_trim`
- `Str_ucase`

#### Floating point

- `ReadFloat`
- `WriteFloat`
- `ShowFPUStack`

#### Random and timing

- `Randomize`
- `Random32`
- `RandomRange`
- `GetTickCount` via `INVOKE`
- `Sleep` via `INVOKE`

#### File I/O

- `CreateOutputFile`
- `OpenInputFile`
- `CloseFile`
- `ReadFromFile`
- `WriteToFile`
- `WriteWindowsMsg`
- `CreateFile` via `INVOKE`
- `ReadFile` via `INVOKE`
- `WriteFile` via `INVOKE`
- `CloseHandle` via `INVOKE`

#### Win32-style console and timing shims via `INVOKE`

- `GetStdHandle`
- `GetConsoleMode`
- `SetConsoleMode`
- `WriteConsole`
- `ReadConsole`
- `FlushConsoleInputBuffer`
- `PeekConsoleInput`
- `ReadConsoleInput`
- `GetNumberOfConsoleInputEvents`
- `GetKeyState`
- `SetConsoleCursorPosition`
- `GetConsoleCursorInfo`
- `SetConsoleCursorInfo`
- `SetConsoleScreenBufferSize`
- `GetConsoleScreenBufferInfo`
- `SetConsoleTitle`
- `GetLocalTime`
- `GetSystemTime`

### Supported macros

- `mWrite`
- `mWriteLn`
- `mWriteString`
- `mWriteSpace`
- `mReadString`
- `mGotoxy`
- `mDump`
- `mDumpMem`
- `mShow`
- `exit`
- user-defined `MACRO` / `ENDM` with positional parameters, `:REQ`, `:=` defaults, nested expansion, `LOCAL` labels, and bare-name / `&param` / `&param&` substitution
- compile-time data-generation directives `REPT` / `REPEAT`, `WHILE`, `FOR`, and `FORC`

## Important limits

This is **not** full ML.EXE/LINK.EXE compatibility yet.

### Not implemented yet

- Full MASM compile-time macro language (`IFB`, `IFIDNI`, `EXITM`, macro-time conditionals, advanced `TEXTEQU` metaprogramming, and similar directives beyond the currently implemented `REPT` / `REPEAT`, `WHILE`, `FOR`, and `FORC` subset)
- Full Win32 API coverage
- GUI procedures such as `MsgBox` / `MsgBoxAsk`
- Full x87 instruction coverage, including the transcendental/environment-management instructions used by the more advanced floating-point support code
- Exact 80-bit `REAL10` arithmetic fidelity; current support accepts/parses `REAL10` and treats it as a practical float-backed approximation for the book's load/store examples
- Full MASM type system, records, anonymous field promotion, and the more advanced structure/union directives beyond the currently supported classroom subset
- The broader typed pointer/cast surface used by the more advanced struct-heavy examples

### Practical caveats

- `ReadKey` works, but in WASI environments it is line-buffered rather than true raw keyboard polling.
- Cursor movement and colors are implemented with ANSI escape sequences; behavior depends on the host terminal.
- `cmd\wasmrun` now supplies real wall-clock and monotonic time to generated WASI modules, so timing/date examples behave more like a native run.
- Relative file paths resolve against the current directory exposed to the WASI runner.
- The latest chapter-10 through chapter-15 survey translated **60 of 66** `.asm` files directly; the remaining 6 are 64-bit listings, Visual C++-generated segment-heavy listings, or a 16-bit DOS sector reader outside the main MASM32/Irvine32 console target.

## Versioning and releases

- The CLI version follows semantic versioning.
- The repository starts at **`0.1.0`**.
- `masm2wasm version` and `masm2wasm --version` show the version embedded into the binary.
- GitHub releases are created by the manually triggered workflow in `.github\workflows\release.yml`.

The release workflow:

1. validates the requested semantic version
2. cross-compiles `masm2wasm` for Linux, Windows, and macOS on `amd64` and `arm64`
3. stamps the release version, commit, and build date into each binary
4. creates a new GitHub release and uploads the binaries plus `SHA256SUMS.txt`

After you initialize the repository on GitHub, trigger it from **Actions** -> **Release** -> **Run workflow**.

## Build

### Native CLI

Users only need the `masm2wasm` binary.

```powershell
go build -o .\dist\masm2wasm.exe .\cmd\masm2wasm
```

### Rebuild the embedded runtime template

Do this after changing `vm\` or `cmd\vmtemplate\`.

```powershell
$env:GOOS='wasip1'
$env:GOARCH='wasm'
$env:CGO_ENABLED='0'
go build -trimpath -ldflags='-s -w' -o .\internal\generator\vmtemplate.wasm .\cmd\vmtemplate
Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED
```

### Build the translator itself as WebAssembly

```powershell
$env:GOOS='wasip1'
$env:GOARCH='wasm'
$env:CGO_ENABLED='0'
go build -trimpath -ldflags='-s -w' -o .\masm2wasm.wasm .\cmd\masm2wasm
Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED
```

## CLI usage

```text
masm2wasm build [options] <source.asm>
masm2wasm run [options] <program.wasm|source.asm> [module args...]
masm2wasm version
```

If no subcommand is provided, `masm2wasm <source.asm>` still behaves like `masm2wasm build <source.asm>`.

### Build a program

```powershell
.\masm2wasm.exe build .\program.asm
.\masm2wasm.exe build .\program.asm -o .\artifacts\program.wasm
```

### Run an already generated WASM module

```powershell
.\masm2wasm.exe run .\program.wasm
.\masm2wasm.exe run .\program.wasm -stdin "5`n"
.\masm2wasm.exe run .\program.wasm -stdin-file .\input.txt
```

### Build and run a MASM source file in one step

```powershell
.\masm2wasm.exe run .\factorial.asm
.\masm2wasm.exe run .\factorial.asm -stdin "5`n"
.\masm2wasm.exe run .\factorial.asm -o .\factorial.wasm
```

### Show the installed version

```powershell
.\masm2wasm.exe version
.\masm2wasm.exe --version
```

### Run the translator itself as WebAssembly

This is the wasm-hosted path. The translator reads MASM source from stdin and writes the produced `.wasm` to stdout.

```powershell
.\masm2wasm.exe run `
  .\masm2wasm.wasm `
  -arg -input -arg - `
  -arg -output -arg - `
  -stdin-file .\program.asm `
  -stdout-file .\helloworld.wasm
```

Then run the generated program:

```powershell
.\masm2wasm.exe run .\helloworld.wasm
```

## File I/O notes

`masm2wasm run` mounts the current host directory into the guest filesystem, so relative file paths like `output.txt` work when you launch the runner from the directory where you want the files to live.

Example:

```powershell
.\masm2wasm.exe build .\somefile.asm -o .\somefile.wasm
.\masm2wasm.exe run .\somefile.wasm
```

If the program opens `output.txt`, it will create or read that file in the current directory.

## What has been tested

The implementation has been exercised with:

- the Easy-MASM `source.asm` sample
- the Easy-MASM console samples for hello world, arithmetic, loops, arrays, input/output, and factorial
- Irvine example programs including `Params.asm`, `proc.asm`, `ArrayFill.asm`, `Flowchart.asm`, `Base-Index.asm`, `Mult.asm`, `Cmpsb.asm`, `Macro1.asm`, `Macro2.asm`, `Macro3.asm`, `RowSum.asm`, `floatTest32.asm`, `MixedMode.asm`, `FCompare.asm`, `Expr.asm`, `LoadAndStore.asm`, `LossOfPrecision.asm`, `IfStatements.asm`, `Struct2.asm`, `Union.asm`, `ShowTime.asm`, `Console1.asm`, `Console2.asm`, `ReadConsole.asm`, `TimingLoop.asm`, `Test_WriteStackFrame.asm`, `PeekInput.asm`, `Keybd.asm`, and `TestReadkey.asm`
- additional real-source translation checks for `Fibon.asm`, `List.asm`, `Repeat.asm`, `IndexOf_asm\indexof.asm`, and `DirectoryListing\asmMain.asm`
- additional Go tests covering procedures, `PROTO`, `USES`, `LOCAL`, user-defined `INVOKE`, indexed addressing, high-level loops, compile-time `REPT` / `WHILE` / `FOR` / `FORC`, string / REP instructions, user-defined macros, struct/union parsing, Win32-style console shims, text aliases, float/x87 execution, comment blocks, jump aliases, file I/O, and generated wasm execution through Wazero
- release-oriented CLI checks covering `masm2wasm version`, `masm2wasm build`, `masm2wasm run <program.wasm>`, and direct `masm2wasm run <source.asm>`
- local release-style cross-compilation for Windows, Linux, and macOS on `amd64` and `arm64`, including version-stamped binaries
