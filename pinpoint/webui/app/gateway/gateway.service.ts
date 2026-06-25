import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import {
  PinpointGateway,
  QueryJobListRequest,
  QueryJobListResponse,
  GetUserInfoRequest,
  GetUserInfoResponse,
  CreateTryJobRequest,
  CreateJobResponse,
  ListBotConfigurationsRequest,
  ListBotConfigurationsResponse,
  ListBenchmarksRequest,
  ListBenchmarksResponse,
  GetBenchmarkInfoRequest,
  GetBenchmarkInfoResponse,
  ListRecentBuildsRequest,
  ListRecentBuildsResponse,
  GetCommitRequest,
  GetCommitResponse,
  GetPatchRequest,
  GetPatchResponse,
} from './gateway';

const HTTP_QUERY_TIMEOUT_MS = 10000;

@Injectable({
  providedIn: 'root',
})
export class GatewayService implements PinpointGateway {
  private http = inject(HttpClient);

  private get<T>(url: string, options?: { params?: any }): Promise<T> {
    return firstValueFrom(
      this.http.get<T>(url, {
        timeout: HTTP_QUERY_TIMEOUT_MS,
        ...options,
      })
    );
  }

  private post<T>(url: string, body: any, options?: { params?: any }): Promise<T> {
    return firstValueFrom(
      this.http.post<T>(url, body, {
        timeout: HTTP_QUERY_TIMEOUT_MS,
        ...options,
      })
    );
  }

  QueryJobList(request: QueryJobListRequest): Promise<QueryJobListResponse> {
    const params = {
      user: request.user,
      configuration: request.configuration,
      jobType: request.jobType,
      'pagination.nextCursor': request.pagination?.nextCursor || '',
      'pagination.prevCursor': request.pagination?.prevCursor || '',
    };
    return this.get('/pinpoint/v1/jobs', { params });
  }

  GetUserInfo(_request: GetUserInfoRequest): Promise<GetUserInfoResponse> {
    return this.get('/pinpoint/v1/user');
  }

  CreateTryJob(request: CreateTryJobRequest): Promise<CreateJobResponse> {
    return this.post('/pinpoint/v1/new', request);
  }

  ListBotConfigurations(
    _request: ListBotConfigurationsRequest
  ): Promise<ListBotConfigurationsResponse> {
    return this.get('/pinpoint/v1/bot-configurations');
  }

  ListBenchmarks(_request: ListBenchmarksRequest): Promise<ListBenchmarksResponse> {
    return this.get('/pinpoint/v1/benchmarks');
  }

  GetBenchmark(request: GetBenchmarkInfoRequest): Promise<GetBenchmarkInfoResponse> {
    return this.get(`/pinpoint/v1/benchmark-info/${encodeURIComponent(request.benchmark)}`);
  }

  ListRecentBuilds(request: ListRecentBuildsRequest): Promise<ListRecentBuildsResponse> {
    return this.get(`/pinpoint/v1/recent-builds/${encodeURIComponent(request.configuration)}`);
  }

  GetCommit(request: GetCommitRequest): Promise<GetCommitResponse> {
    return this.get(`/pinpoint/v1/commit/${encodeURIComponent(request.commit)}`);
  }

  GetPatch(request: GetPatchRequest): Promise<GetPatchResponse> {
    const params: Record<string, string | number> = {
      host: request.host,
      change: request.change,
    };
    if (request.patchset !== undefined && request.patchset !== null) {
      params.patchset = request.patchset;
    }
    return this.get('/pinpoint/v1/patch', { params });
  }
}
