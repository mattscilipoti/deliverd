# deliverd

A terminal tool for checking whether a delivery fits in a vehicle. Enter sets of items (length, width, height, optional weight, quantity) and it shows the volume used and left, whether each item can fit at all, whether the load is within the weight limit, and a 3D packing layout you can view in the terminal or open as an image.

It ships with the cargo area of a 2013 Toyota Prius v, in inches and pounds, and is meant to be a quick answer, not a guarantee. See [Assumptions and limits](#assumptions-and-limits).

## Build and run

Requires Go (see `go.mod`).

```
go run .                 # run from source
./build.sh               # static Linux binary: dist/deliverd-linux-amd64
./build.sh arm64         # ...or arm64
```

`build.sh` stamps the build time into the binary; it shows on the main screen, in help, and with `-version`. A `go run` build says "dev build (not stamped)".

To install on a Debian machine: `sudo install -m 755 dist/deliverd-linux-amd64 /usr/local/bin/deliverd`

## Using it

Type L, W, H (inches), an optional weight in lb per item, and a quantity (blank means 1), then Enter. After each set it updates:

- **Volume** used and available.
- **Size check**: every item must fit the cargo area in some rotation.
- **Weight**: total against the limit for people and cargo, minus the occupants' weight.
- **Packing**: a heuristic that places heavier pieces low and forward, never on a lighter piece, and with enough support underneath.

Press `Esc` for command mode, then `?` for the full key list. The essentials:

| Key | Does |
|---|---|
| `Tab` / `Enter` | next field / add the set |
| `Esc` | command mode |
| `↑` `↓`, `e`, `d`, `c` | select, edit, delete a set, clear all |
| `r`, `v`, `s` | rear seats up/down, next vehicle, custom cargo size |
| `p` | weight limit and occupant weight |
| `D` | dolly: off, behind the seats (not counted), or with the items (packed) |
| `m` | merge identical sets on/off |
| `w`, `W`, `o` | save, save as, open a saved load |
| `L` | layout: top, driver side, passenger side and rear views |
| `q` | quit |

On the layout screen: `s` searches longer for a better layout (any key stops it and keeps the best so far), `t` sets how long, `i` saves the layout as an SVG image and opens it in your viewer, and `D` toggles the dolly.

## Flags

| Flag | Meaning |
|---|---|
| `-seats up\|down` | rear seats (default down) |
| `-length -width -height` | override the cargo size, inches |
| `-payload LB`, `-occupants LB` | override the weight limit and occupant weight |
| `-dolly off\|seats\|items` | where the dolly goes |
| `-merge=false` | keep identical sets separate |
| `-dir PATH` | where saved loads live |
| `-image-dir PATH` | where layout images are saved (default: temp dir) |
| `-search-time DUR` | how long the layout search runs (default 10s) |
| `-version` | print the build time and exit |

## Files it writes

- **Saved loads**: one JSON file each in `~/.config/deliverd/loads/` (or `-dir`). Item sets and weights only; the vehicle, seats and dolly are not saved. In the dev container this folder is a persisted volume, so it survives rebuilds.
- **Layout images**: `deliverd-layout-<time>.svg` in the temp directory (or `-image-dir`).

## Assumptions and limits

- **Vehicle data** (`vehicles.go`, 2013 Toyota Prius v):
  - Rear seats up: 38 × 42 × 31 in, from owner measurements on a 2016 Prius v (same generation) reported on PriusChat.
  - Rear seats down: width and height from the same source; **the 89 in length is an estimate**, solved so the box matches Toyota's listed 67.3 ft³. Replace it with a measured floor length if you have one.
  - Weight limit: 915 lb for occupants plus cargo, from the driver's door plaque. Occupants default to 175 lb and come out of that limit. The dolly's weight is not counted separately; add it to the occupant weight.
- **Left-hand drive** is assumed in the driver and passenger views.
- **The packing is a heuristic.** "Couldn't place" means no layout was found, not that none exists, and "found a layout" is not a guarantee. It does not model crushing, fragile items or items that must stay upright. The quick packer handles up to 150 pieces and the layout search up to 400.
- The cargo area is treated as a plain box; wheel wells and sloping surfaces are not modelled.

## Development

```
go vet ./... && go test ./...
go test -race ./...      # the layout search runs in a goroutine
```

| File | Contents |
|---|---|
| `cargo.go` | volume, weight and merge logic |
| `pack.go`, `pack_search.go` | the packing heuristic and the longer annealing search |
| `layout.go`, `svg.go` | terminal layout views and the SVG export |
| `vehicles.go` | vehicle presets and their sources |
| `store.go` | saved loads |
| `tui*.go`, `main.go` | the Bubble Tea interface and flags |
