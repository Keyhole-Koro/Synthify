import { api } from "@electron-forge/core";

const command = process.argv[2];

switch (command) {
  case "start":
    await api.start({ dir: process.cwd(), interactive: false });
    break;
  case "package":
    await api.package({ dir: process.cwd(), interactive: false });
    break;
  case "make":
    await api.make({ dir: process.cwd(), interactive: false });
    break;
  default:
    throw new Error(`Unknown Forge command: ${command ?? "<missing>"}`);
}
