"""`kmise` command-line interface."""
from __future__ import annotations

from pathlib import Path

import click

from . import protocol as p
from .audio import AudioError, load_pcm
from .device import Device, DeviceError


def fmt_time(seconds: float) -> str:
    m, s = divmod(seconds, 60)
    return f"{int(m)}:{s:05.2f}"


def parse_slots(values: tuple[str, ...]) -> list[int]:
    """Accept '3', '3,7', '10-15'. Slots are 0-based, as on the device."""
    out: list[int] = []
    for v in values:
        for part in v.split(","):
            if "-" in part:
                a, b = part.split("-", 1)
                out.extend(range(int(a), int(b) + 1))
            elif part:
                out.append(int(part))
    bad = [s for s in out if not 0 <= s < p.NUM_SLOTS]
    if bad:
        raise click.BadParameter(f"slots must be 0-{p.NUM_SLOTS - 1}: {bad}")
    return sorted(set(out))


def progress_bar(label: str):
    bar = click.progressbar(length=1, label=label, width=30, show_percent=True)

    def update(done: int, total: int) -> None:
        bar.length = total
        bar.update(done - bar.pos)

    return bar, update


@click.group()
@click.version_option()
def main() -> None:
    """Talk to the YUER XS04 LOOP&DRUM (KMISE / lotmusic / Flamma / Donner) looper."""


def open_device() -> Device:
    try:
        return Device()
    except DeviceError as e:
        raise click.ClickException(str(e)) from e


@main.command()
def info() -> None:
    """Show firmware version and slot usage."""
    with open_device() as d:
        slots = d.list_slots()
        used = [s for s in slots if not s.empty]
        click.echo(f"Device:   XS04 LOOP&DRUM (version {d.version()!r})")
        click.echo(f"Slots:    {len(used)} of {len(slots)} used")
        click.echo(f"Audio:    {fmt_time(sum(s.seconds for s in used))} total")


@main.command("list")
@click.option("--all", "show_all", is_flag=True, help="Include empty slots.")
def list_cmd(show_all: bool) -> None:
    """List recorded slots."""
    with open_device() as d:
        slots = d.list_slots()
    shown = [s for s in slots if show_all or not s.empty]
    if not shown:
        click.echo("No recordings on the device.")
        return
    for s in shown:
        size = f"{s.pcm_bytes / 1e6:6.1f} MB" if not s.empty else "   empty"
        click.echo(f"  {s.index:2d}  {fmt_time(s.seconds):>9}  {size}")


@main.command()
@click.argument("slots", nargs=-1)
@click.option("--all", "export_all", is_flag=True, help="Export every non-empty slot.")
@click.option("-o", "--out", "out_dir", type=click.Path(file_okay=False, path_type=Path),
              default=Path("."), show_default=True, help="Output directory.")
@click.option("--prefix", default="slot", show_default=True, help="File name prefix.")
def export(slots: tuple[str, ...], export_all: bool, out_dir: Path, prefix: str) -> None:
    """Export slots to 24-bit WAV files. SLOTS: e.g. 3  3,7  10-15."""
    if not slots and not export_all:
        raise click.UsageError("give slot numbers or --all")
    out_dir.mkdir(parents=True, exist_ok=True)
    with open_device() as d:
        listing = {s.index: s for s in d.list_slots()}
        wanted = [i for i, s in listing.items() if not s.empty] if export_all else parse_slots(slots)
        if not wanted:
            click.echo("Nothing to export.")
            return
        for i in wanted:
            if listing[i].empty:
                click.echo(f"  slot {i:2d}: empty, skipped")
                continue
            path = out_dir / f"{prefix}_{i:02d}.wav"
            bar, update = progress_bar(f"  slot {i:2d}")
            with bar:
                pcm = d.read_slot(i, update)
            path.write_bytes(p.wav_header(len(pcm)) + pcm)
            click.echo(f"  -> {path}  ({fmt_time(len(pcm) / p.FRAME_BYTES / p.SAMPLE_RATE)})")


@main.command("import")
@click.argument("slot", type=int)
@click.argument("file", type=click.Path(exists=True, dir_okay=False, path_type=Path))
@click.option("--force", is_flag=True, help="Overwrite a non-empty slot.")
def import_cmd(slot: int, file: Path, force: bool) -> None:
    """Import an audio FILE into SLOT."""
    parse_slots((str(slot),))
    try:
        pcm = load_pcm(file)
    except AudioError as e:
        raise click.ClickException(str(e)) from e
    if len(pcm) > p.MAX_IMPORT_BYTES:
        raise click.ClickException("audio is longer than the 240 s limit")
    with open_device() as d:
        current = {s.index: s for s in d.list_slots()}[slot]
        if not current.empty and not force:
            raise click.ClickException(
                f"slot {slot} already holds {fmt_time(current.seconds)}; use --force to overwrite"
            )
        bar, update = progress_bar(f"  slot {slot:2d}")
        with bar:
            d.write_slot(slot, pcm, update)
        after = {s.index: s for s in d.list_slots()}[slot]
    click.echo(f"  imported {file.name} -> slot {slot} ({fmt_time(after.seconds)} on device)")


@main.command()
@click.argument("slots", nargs=-1)
@click.option("--all", "delete_all", is_flag=True, help="Delete EVERY slot.")
@click.option("-y", "--yes", is_flag=True, help="Don't ask for confirmation.")
def delete(slots: tuple[str, ...], delete_all: bool, yes: bool) -> None:
    """Delete slots. SLOTS: e.g. 3  3,7  10-15."""
    if not slots and not delete_all:
        raise click.UsageError("give slot numbers or --all")
    targets = list(range(p.NUM_SLOTS)) if delete_all else parse_slots(slots)
    what = "ALL slots" if delete_all else f"slot(s) {', '.join(map(str, targets))}"
    if not yes:
        click.confirm(f"Permanently delete {what}?", abort=True)
    with open_device() as d:
        d.delete(targets)
    click.echo(f"Deleted {what}.")


if __name__ == "__main__":
    main()
