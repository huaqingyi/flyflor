"""Message bus module for decoupled channel-agent communication."""

from flyflor.bus.events import InboundMessage, OutboundMessage
from flyflor.bus.queue import MessageBus

__all__ = ["MessageBus", "InboundMessage", "OutboundMessage"]
