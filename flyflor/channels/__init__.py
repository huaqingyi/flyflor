"""Chat channels module with plugin architecture."""

from flyflor.channels.base import BaseChannel
from flyflor.channels.manager import ChannelManager

__all__ = ["BaseChannel", "ChannelManager"]
