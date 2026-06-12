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
} from './gateway';

@Injectable({
  providedIn: 'root',
})
export class GatewayService implements PinpointGateway {
  private http = inject(HttpClient);

  QueryJobList(request: QueryJobListRequest): Promise<QueryJobListResponse> {
    const params = {
      user: request.user,
      configuration: request.configuration,
      jobType: request.jobType,
      'pagination.nextCursor': request.pagination?.nextCursor || '',
      'pagination.prevCursor': request.pagination?.prevCursor || '',
    };
    return firstValueFrom(this.http.get<QueryJobListResponse>('/pinpoint/v1/jobs', { params }));
  }

  GetUserInfo(_request: GetUserInfoRequest): Promise<GetUserInfoResponse> {
    return firstValueFrom(this.http.get<GetUserInfoResponse>('/pinpoint/v1/user'));
  }

  CreateTryJob(request: CreateTryJobRequest): Promise<CreateJobResponse> {
    return firstValueFrom(this.http.post<CreateJobResponse>('/pinpoint/v1/new', request));
  }

  ListBotConfigurations(
    _request: ListBotConfigurationsRequest
  ): Promise<ListBotConfigurationsResponse> {
    return firstValueFrom(
      this.http.get<ListBotConfigurationsResponse>('/pinpoint/v1/bot-configurations')
    );
  }

  ListBenchmarks(_request: ListBenchmarksRequest): Promise<ListBenchmarksResponse> {
    return firstValueFrom(this.http.get<ListBenchmarksResponse>('/pinpoint/v1/benchmarks'));
  }

  GetBenchmark(request: GetBenchmarkInfoRequest): Promise<GetBenchmarkInfoResponse> {
    return firstValueFrom(
      this.http.get<GetBenchmarkInfoResponse>(
        `/pinpoint/v1/benchmark-info/${encodeURIComponent(request.benchmark)}`
      )
    );
  }

  ListRecentBuilds(request: ListRecentBuildsRequest): Promise<ListRecentBuildsResponse> {
    return firstValueFrom(
      this.http.get<ListRecentBuildsResponse>(
        `/pinpoint/v1/recent-builds/${encodeURIComponent(request.configuration)}`
      )
    );
  }
}
