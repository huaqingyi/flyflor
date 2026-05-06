from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol


@dataclass(frozen=True)
class BridgeMessage:
    role: str
    content: str


class Bridge(Protocol):
    name: str

    def start(self, task_id: int) -> None:
        """Start a bridge session for a task."""

    def send(self, message: BridgeMessage) -> None:
        """Send one structured message into the bridge."""

    def interrupt(self) -> None:
        """Interrupt the running bridge session."""

    def close(self) -> None:
        """Close the bridge session and release resources."""
