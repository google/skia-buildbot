ARG git_base="gcr.io/skia-public/git:testing-slim"

FROM $git_base AS setup

# Create the workdir.
USER root
RUN mkdir /work && chown skia:skia /work
USER skia:skia
WORKDIR /work

# Sync the given repo to the requested revision.
# TODO(borenet): It'd be nice to support something more granular than the Docker
# cache, but I'm not sure how we'd do that in a way that doesn't defeat the
# purpose of using Docker at all.
ARG REPO
ARG REVISION
ENV REPO=$REPO
ENV REVISION=$REVISION
RUN git clone ${REPO} \
  && cd $(basename ${REPO} | sed -e 's/\.git//') \
  && git reset --hard ${REVISION}

# If provided, apply the given patch ref. Note that PATCH_REF is not referenced
# above this point, which allows the image with just repo+revision to be reused
# as needed from the Docker cache.
ARG PATCH_REF
ENV PATCH_REV=$PATCH_REF
RUN if [ -n "$PATCH_REF" ]; then \
  cd $(basename ${REPO} | sed -e 's/\.git//') \
  && git fetch ${REPO} ${PATCH_REF} \
  && git checkout FETCH_HEAD \
  && git rebase ${REVISION} \
; fi

# Remove the .git directory, because its contents may be different even if the
# checkout is synced to the same revision. That prevents task deduplication.
RUN rm -rf /work/$(basename ${REPO} | sed -e 's/\.git//')/.git

FROM gcr.io/skia-public/basedebian:testing-slim AS git-checkout
COPY --from=setup /work /work
