'use client';

import React, { useEffect, useState, useMemo } from 'react';
import { createJobLogDataSource } from '@/lib/data-source';
import { JobLogViewer } from '../JobLogViewer';
import { JobTrace } from './JobTrace';
import type { JobSummary, ListJobsResponse } from '../types';

interface AuditPageProps {
  embedded?: boolean;
  forcedJobId?: string | null;
  onJobSelect?: (jobId: string) => void;
}

export function AuditPage({ embedded = false, forcedJobId, onJobSelect }: AuditPageProps) {
  const [jobs, setJobs] = useState<JobSummary[]>([]);
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [mainTab, setMainTab] = useState<'logs' | 'trace'>('logs');
  const dataSource = useMemo(() => createJobLogDataSource(), []);

  useEffect(() => {
    fetch('/api/jobs')
      .then((r) => r.json())
      .then((data: Partial<ListJobsResponse>) => setJobs(Array.isArray(data.jobs) ? data.jobs : []))
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  // 外部からジョブIDが指定されたら選択状態を同期
  useEffect(() => {
    if (forcedJobId) {
      setSelectedJobId(forcedJobId);
      setMainTab('logs');
    }
  }, [forcedJobId]);

  function selectJob(jobId: string) {
    setSelectedJobId(jobId);
    setMainTab('logs');
    onJobSelect?.(jobId);
  }

  const filteredJobs = useMemo(() => {
    const q = searchQuery.toLowerCase();
    return jobs.filter(
      (j) => j.jobId.toLowerCase().includes(q) || j.documentId.toLowerCase().includes(q),
    );
  }, [jobs, searchQuery]);

  const selectedJob = useMemo(
    () => jobs.find((j) => j.jobId === selectedJobId) ?? null,
    [jobs, selectedJobId],
  );

  if (loading) return <div className="p-10 text-stone-400 text-sm">Loading...</div>;

  return (
    <div className={`flex bg-stone-50 overflow-hidden ${embedded ? 'h-full' : 'h-screen'}`}>
      {/* Sidebar */}
      <div className="w-80 border-r border-stone-200 bg-white flex flex-col">
        <div className="p-4 border-b border-stone-100 bg-stone-50/50">
          <div className="flex items-center justify-between mb-1">
            <h1 className="text-sm font-bold tracking-tight text-stone-800 uppercase">Log Viewer</h1>
            {!embedded && (
              <a
                href="/dashboards"
                className="text-[10px] px-2 py-0.5 rounded bg-stone-100 text-stone-500 hover:bg-stone-200 transition-colors font-medium"
              >
                BI
              </a>
            )}
          </div>
          <p className="text-[10px] text-stone-400 font-medium">Document Processing History</p>
          <div className="mt-4 relative">
            <input
              type="text"
              placeholder="Search by Job ID or Doc ID..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-8 pr-3 py-1.5 text-[11px] bg-white border border-stone-200 rounded-md focus:outline-none focus:ring-2 focus:ring-stone-300 focus:border-stone-400"
            />
            <svg className="absolute left-2.5 top-2 h-3.5 w-3.5 text-stone-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
        </div>
        <div className="flex-1 overflow-y-auto">
          {filteredJobs.length === 0 ? (
            <div className="p-10 text-center text-[11px] text-stone-400 italic">No jobs found</div>
          ) : (
            filteredJobs.map((job) => (
              <button
                key={job.jobId}
                onClick={() => selectJob(job.jobId)}
                className={`w-full text-left p-4 border-b border-stone-50 transition-colors hover:bg-stone-50 ${selectedJobId === job.jobId ? 'bg-stone-100' : ''}`}
              >
                <div className="flex justify-between items-start mb-1">
                  <span className="text-[10px] font-mono text-stone-400">{job.jobId.slice(0, 8)}...</span>
                  <span className={`text-[9px] px-1.5 py-0.5 rounded-full font-bold uppercase ${
                    job.status === 3 ? 'bg-emerald-100 text-emerald-700' :
                    job.status === 4 ? 'bg-red-100 text-red-700' :
                    'bg-amber-100 text-amber-700'
                  }`}>
                    {job.status === 1 ? 'Queued' : job.status === 2 ? 'Running' : job.status === 3 ? 'Done' : 'Failed'}
                  </span>
                </div>
                <div className="text-xs font-bold text-stone-700 truncate">{job.documentId}</div>
                <div className="text-[10px] text-stone-400 mt-1">{new Date(job.createdAt).toLocaleString()}</div>
              </button>
            ))
          )}
        </div>
      </div>

      {/* Main */}
      <div className="flex-1 flex flex-col overflow-hidden bg-white">
        {selectedJob ? (
          <>
            {/* Logs / Trace tab bar */}
            <div className="flex items-center gap-1 px-4 pt-3 pb-0 border-b border-stone-200 shrink-0">
              {(['logs', 'trace'] as const).map((t) => (
                <button
                  key={t}
                  onClick={() => setMainTab(t)}
                  className={`px-3 py-1.5 text-xs font-semibold rounded-t transition-colors capitalize ${
                    mainTab === t
                      ? 'bg-white border border-b-white border-stone-200 -mb-px text-stone-800'
                      : 'text-stone-400 hover:text-stone-600'
                  }`}
                >
                  {t}
                </button>
              ))}
            </div>

            <div className="flex-1 overflow-hidden">
              {mainTab === 'logs' ? (
                <JobLogViewer
                  jobId={selectedJob.jobId}
                  documentId={selectedJob.documentId}
                  workspaceId={selectedJob.workspaceId}
                  dataSource={dataSource}
                />
              ) : (
                <JobTrace jobId={selectedJob.jobId} />
              )}
            </div>
          </>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center text-stone-300 p-10 text-center">
            <svg className="w-12 h-12 mb-4 opacity-20" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
            </svg>
            <p className="text-sm font-medium">Select a job to view its logs.</p>
          </div>
        )}
      </div>
    </div>
  );
}
