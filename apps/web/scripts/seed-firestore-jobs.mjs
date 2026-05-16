import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { initializeApp, getApps } from 'firebase-admin/app';
import { getFirestore } from 'firebase-admin/firestore';

loadEnvFile('.env.local');

const projectId = readEnv('NEXT_PUBLIC_FIREBASE_PROJECT_ID', 'demo-synthify');
const emulatorHost = readEnv('NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_HOST', '127.0.0.1');
const emulatorPort = readEnv('NEXT_PUBLIC_FIREBASE_FIRESTORE_EMULATOR_PORT', '8282');

process.env.FIRESTORE_EMULATOR_HOST = `${emulatorHost}:${emulatorPort}`;

if (!getApps().length) {
  initializeApp({ projectId });
}

const db = getFirestore();

const now = new Date();
const minutesAgo = (minutes) => new Date(now.getTime() - minutes * 60_000).toISOString();

const jobs = [
  {
    workspaceId: 'ws_seed_1',
    jobId: 'job-audit-demo-1',
    jobType: 'process_document',
    documentId: 'doc_seed_1',
    treeId: 'tree_ws_seed_1',
    status: 'succeeded',
    currentStage: '',
    progress: 100,
    message: 'Completed',
    errorMessage: '',
    createdAt: minutesAgo(70),
    startedAt: minutesAgo(68),
    updatedAt: minutesAgo(55),
    completedAt: minutesAgo(55),
  },
  {
    workspaceId: 'ws_seed_1',
    jobId: 'job-audit-demo-2',
    jobType: 'process_document',
    documentId: 'doc_seed_2',
    treeId: 'tree_ws_seed_1',
    status: 'running',
    currentStage: 'semantic_chunking',
    progress: 35,
    message: 'Chunking document',
    errorMessage: '',
    createdAt: minutesAgo(30),
    startedAt: minutesAgo(29),
    updatedAt: minutesAgo(1),
  },
  {
    workspaceId: 'ws_seed_1',
    jobId: 'job-audit-demo-3',
    jobType: 'process_document',
    documentId: 'doc_seed_3',
    treeId: 'tree_ws_seed_1',
    status: 'failed',
    currentStage: '',
    progress: 90,
    message: 'Failed',
    errorMessage: 'Unable to parse source document',
    createdAt: minutesAgo(120),
    startedAt: minutesAgo(119),
    updatedAt: minutesAgo(110),
    completedAt: minutesAgo(110),
  },
];

for (const job of jobs) {
  await db.doc(`workspaces/${job.workspaceId}/jobs/${job.jobId}`).set(job, { merge: true });
}

console.log(`Seeded ${jobs.length} Firestore job documents into ${projectId} (${emulatorHost}:${emulatorPort})`);

function readEnv(key, fallback) {
  const value = process.env[key];
  return typeof value === 'string' && value !== '' ? value : fallback;
}

function loadEnvFile(filename) {
  const filePath = path.resolve(process.cwd(), filename);
  if (!fs.existsSync(filePath)) return;
  const raw = fs.readFileSync(filePath, 'utf8');
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq <= 0) continue;
    const key = trimmed.slice(0, eq).trim();
    if (process.env[key]) continue;
    let value = trimmed.slice(eq + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    process.env[key] = value;
  }
}
