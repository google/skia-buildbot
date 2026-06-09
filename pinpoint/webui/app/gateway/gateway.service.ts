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
}
