import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { createAppRouter, queryClient } from '@/client/router';
import { initNewRelicBrowser } from '@/lib/newrelic/browser';
import '@/index.css';
import '@/paper-in-paper.css';

try {
  initNewRelicBrowser();
} catch (error) {
  console.error('New Relic Browser initialization failed:', error);
}

const router = createAppRouter();

const rootElement = document.getElementById('root');
if (rootElement && !rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement);
  root.render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </React.StrictMode>,
  );
}
