/**
 * This file is generated from contracts/openapi/log-viewer.yaml by openapi-typescript.
 * Regenerate with `bun run gen:openapi`.
 */

export interface paths {
  "/api/jobs": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["listJobs"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/api/jobs/{jobId}/logs": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["listJobLogs"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/api/jobs/search": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post: operations["searchJobLogs"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/api/jobs/related": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post: operations["listRelatedJobLogs"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
}

export type webhooks = Record<string, never>;

export interface components {
  schemas: {
    ErrorResponse: {
      error: string;
    };
    JobSummary: {
      jobId: string;
      documentId: string;
      workspaceId: string;
      jobType: string;
      status: number;
      currentStage: string;
      errorMessage: string;
      retryCount: number;
      createdAt: string;
      updatedAt: string;
    };
    ListJobsResponse: {
      jobs: components["schemas"]["JobSummary"][];
    };
    JobLog: {
      timestamp: string;
      level: string;
      event: string;
      message: string;
      detailJson: string;
      source: string;
      sourceId: string;
      jobId: string;
      documentId: string;
      workspaceId: string;
    };
    ListJobLogsResponse: {
      logs: components["schemas"]["JobLog"][];
      nextPageToken: string;
    };
    SearchJobLogsRequest: {
      query?: string;
      workspaceId?: string;
      documentId?: string;
      jobId?: string;
      levels?: string[];
      events?: string[];
      fromTimestamp?: string;
      toTimestamp?: string;
      pageToken?: string;
      limit?: number;
    };
    RelatedScope: "job" | "document" | "workspace";
    ListRelatedJobLogsRequest: {
      scope: components["schemas"]["RelatedScope"];
      jobId?: string;
      documentId?: string;
      workspaceId?: string;
      pageToken?: string;
      limit?: number;
    };
    RelatedJob: {
      jobId: string;
      status: number;
      createdAt: string;
      logs: components["schemas"]["JobLog"][];
    };
    RelatedJobGroup: {
      workspaceId: string;
      documentId: string;
      jobs: components["schemas"]["RelatedJob"][];
    };
    ListRelatedJobLogsResponse: {
      groups: components["schemas"]["RelatedJobGroup"][];
      nextPageToken: string;
    };
  };
  responses: never;
  parameters: never;
  requestBodies: never;
  headers: never;
  pathItems: never;
}

export type $defs = Record<string, never>;

export interface operations {
  listJobs: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListJobsResponse"];
        };
      };
      500: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListJobsResponse"];
        };
      };
    };
  };
  listJobLogs: {
    parameters: {
      query?: {
        page_token?: string;
        limit?: number;
      };
      header?: never;
      path: {
        jobId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListJobLogsResponse"];
        };
      };
      400: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ErrorResponse"];
        };
      };
      500: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListJobLogsResponse"];
        };
      };
    };
  };
  searchJobLogs: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["SearchJobLogsRequest"];
      };
    };
    responses: {
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListJobLogsResponse"];
        };
      };
      400: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ErrorResponse"];
        };
      };
      500: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListJobLogsResponse"];
        };
      };
    };
  };
  listRelatedJobLogs: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["ListRelatedJobLogsRequest"];
      };
    };
    responses: {
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListRelatedJobLogsResponse"];
        };
      };
      400: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ErrorResponse"];
        };
      };
      500: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ListRelatedJobLogsResponse"];
        };
      };
    };
  };
}
