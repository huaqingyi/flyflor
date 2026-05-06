from flyflor_core.config import Config
from flyflor_core.server import run_server


def main() -> None:
    config = Config.from_env()
    run_server(config)


if __name__ == "__main__":
    main()
