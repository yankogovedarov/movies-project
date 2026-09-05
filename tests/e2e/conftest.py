import os
import subprocess
import time
import urllib.request
import urllib.error
import pytest

BASE_URL = "http://localhost:8080"
REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
EXE_PATH = os.path.join(REPO_ROOT, "movietracker_e2e.exe")


def _kill_port_8080():
    subprocess.run(
        [
            "powershell", "-Command",
            "Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue"
            " | Select-Object -ExpandProperty OwningProcess"
            " | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }",
        ],
        capture_output=True,
    )
    time.sleep(0.5)


def _wait_for_server(timeout: int = 30) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            urllib.request.urlopen(BASE_URL, timeout=2)
            return True
        except Exception:
            time.sleep(1)
    return False


@pytest.fixture(scope="session", autouse=True)
def live_server():
    # Build binary
    result = subprocess.run(
        ["go", "build", "-o", EXE_PATH, "./cmd/movietracker"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, f"go build failed:\n{result.stderr}"

    _kill_port_8080()

    # Bug 24: тестовете не бива да оставят отворени прозорци след себе си.
    # Флагът спира пускането на VLC, Explorer и авто-отварянето на браузъра.
    env = dict(os.environ, MOVIETRACKER_NO_LAUNCH="1")

    proc = subprocess.Popen(
        [EXE_PATH],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        env=env,
    )

    if not _wait_for_server(timeout=30):
        proc.kill()
        raise RuntimeError("Server did not become ready within 30 seconds")

    yield BASE_URL

    proc.kill()
    proc.wait(timeout=5)
    try:
        os.remove(EXE_PATH)
    except OSError:
        pass


# Bug 23: филтрите и сортирането вече се помнят в базата, а сървърът и базата са
# общи за цялата сесия. Затова преди всеки тест състоянието се връща към
# стойностите по подразбиране — иначе филтър, оставен от предишен тест, би текъл
# в следващия, който отваря голо "/".
DEFAULT_STATE_URL = (
    BASE_URL + "/?status=all&disk=on&sort=name&dir=asc&q=&trans=all&del=all"
)


@pytest.fixture(autouse=True)
def reset_prefs(live_server):
    urllib.request.urlopen(DEFAULT_STATE_URL, timeout=5).read()
    yield
