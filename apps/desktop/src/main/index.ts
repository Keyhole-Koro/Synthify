/**
 * Desktop application main process lifecycle manager.
 * Initializes the SQLite repository, registers IPC channels, creates the primary
 * browser window, and enforces security sandboxing.
 *
 * @module
 */
import path from "node:path";
import { app, BrowserWindow, session } from "electron";
import { AgentRuntimeManager } from "./agent-runtime-manager.js";
import { registerIpc } from "./ipc.js";
import { SqliteProjectRepository } from "./project-repository.js";

let mainWindow: BrowserWindow | null = null;
let disposeIpc: (() => void) | null = null;
let projects: SqliteProjectRepository | null = null;
let agents: AgentRuntimeManager | null = null;

app.enableSandbox();

/**
 * Creates and configures the main desktop BrowserWindow.
 * Enforces navigation restrictions, security policies, and loads the Vite bundle.
 */
function createWindow(): void {
  mainWindow = new BrowserWindow({
    width: 1440,
    height: 920,
    minWidth: 1040,
    minHeight: 680,
    backgroundColor: "#0c0d10",
    titleBarStyle: process.platform === "darwin" ? "hiddenInset" : "default",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
      webSecurity: true,
    },
  });

  mainWindow.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  mainWindow.webContents.on("will-navigate", (event, url) => {
    const current = mainWindow?.webContents.getURL();
    if (current && url !== current) event.preventDefault();
  });

  if (MAIN_WINDOW_VITE_DEV_SERVER_URL) {
    void mainWindow.loadURL(MAIN_WINDOW_VITE_DEV_SERVER_URL);
  } else {
    void mainWindow.loadFile(path.join(__dirname, `../renderer/${MAIN_WINDOW_VITE_NAME}/index.html`));
  }

  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

void app.whenReady().then(() => {
  session.defaultSession.setPermissionRequestHandler((_webContents, _permission, callback) => callback(false));
  projects = new SqliteProjectRepository(path.join(app.getPath("userData"), "insightify.sqlite3"));
  agents = new AgentRuntimeManager(projects, () => mainWindow);
  disposeIpc = registerIpc({ projects, agents, window: () => mainWindow });
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});

app.on("before-quit", () => {
  disposeIpc?.();
  disposeIpc = null;
  void agents?.stop();
  agents = null;
  projects?.close();
  projects = null;
});
