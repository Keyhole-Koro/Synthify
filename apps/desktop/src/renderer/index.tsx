/**
 * Web application entry point for the desktop renderer process.
 * Mounts the top-level App component into the HTML root container in React StrictMode.
 *
 * @module
 */
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./styles.css";

const root = document.getElementById("root");
if (!root) throw new Error("Missing root element");

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

