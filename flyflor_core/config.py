from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Config:
    host: str
    port: int
    home: Path
    database: Path
    qdrant_url: str
    qdrant_collection: str

    @classmethod
    def from_env(cls) -> "Config":
        home = Path(os.getenv("FLYFLOR_HOME", "./data/flyflor")).expanduser()
        database = Path(os.getenv("FLYFLOR_DATABASE", str(home / "flyflor.db"))).expanduser()
        return cls(
            host=os.getenv("FLYFLOR_HOST", "127.0.0.1"),
            port=int(os.getenv("FLYFLOR_PORT", "8080")),
            home=home,
            database=database,
            qdrant_url=os.getenv("QDRANT_URL", "http://localhost:6333"),
            qdrant_collection=os.getenv("QDRANT_COLLECTION", "flyflor_memory"),
        )
