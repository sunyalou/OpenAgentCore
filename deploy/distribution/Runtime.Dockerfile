# Each input is an immutable Linux amd64 image built from the same Core revision.
# Reuse the native packages from existing profiles.
ARG CODEX_IMAGE
ARG CLAUDE_IMAGE
ARG MCODE_IMAGE
FROM ${CODEX_IMAGE} AS codex
FROM ${CLAUDE_IMAGE} AS claude
FROM ${MCODE_IMAGE}

# XCCL workloads need the RDMA user space and a pinned MPICH launcher. Kernel
# drivers and firmware stay on the node host; only user space ships here. The
# build context carries mpich.tar.gz.
USER root
COPY mpich.tar.gz /tmp/mpich.tar.gz
RUN apt-get update && apt-get install -y --no-install-recommends \
      libpciaccess0 libibverbs1 ibverbs-providers librdmacm1 ibverbs-utils \
      libibverbs-dev librdmacm-dev openssh-client \
    && rm -rf /var/lib/apt/lists/* \
    && tar -xzf /tmp/mpich.tar.gz -C /opt \
    && rm /tmp/mpich.tar.gz \
    && printf '/opt/mpich/lib\n' > /etc/ld.so.conf.d/mpich.conf \
    && ldconfig
ENV PATH=/opt/mpich/bin:$PATH

# XCCL builds run inside the sandbox: the C/C++ toolchain, wget for the
# build's pinned dependency downloads, and preset paths that keep the build
# from cloning rdma-core, gtest and gflags over the internal git.
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential cmake pkg-config wget zlib1g-dev libgtest-dev libgflags-dev \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/xccl-deps/gtest/lib /opt/xccl-deps/gtest/include \
                /opt/xccl-deps/gflags/lib /opt/xccl-deps/gflags/include \
    && ln -s /usr/lib/x86_64-linux-gnu/libgtest.a /opt/xccl-deps/gtest/lib/libgtest.a \
    && ln -s /usr/include/gtest /opt/xccl-deps/gtest/include/gtest \
    && ln -s /usr/lib/x86_64-linux-gnu/libgflags.a /opt/xccl-deps/gflags/lib/libgflags.a \
    && ln -s /usr/include/gflags /opt/xccl-deps/gflags/include/gflags
ENV CMAKE_PATH=/usr \
    GTEST_PATH=/opt/xccl-deps/gtest \
    GFLAGS_PATH=/opt/xccl-deps/gflags \
    USE_SYSTEM_RDMA=ON

# Keep the shared daemon and dependencies from the MiniMax base.
# Native harness packages remain outside the workspace.
COPY --from=codex /usr/local/bin/codex /usr/local/bin/codex
COPY --from=codex /usr/local/codex-resources /usr/local/codex-resources
COPY --from=claude /opt/claude-sdk /opt/claude-sdk

ENV OAC_RUNTIME_CODEX_BIN=/usr/local/bin/codex \
    OAC_RUNTIME_CLAUDE_SDK_NODE=/usr/local/bin/node \
    OAC_RUNTIME_CLAUDE_SDK_ENTRYPOINT=/opt/claude-sdk/dist/main.js \
    OAC_RUNTIME_CLAUDE_SDK_WORKSPACE=managed

USER 1000:1000
RUN test "$(codex --version)" = "codex-cli 0.153.4" \
    && node /opt/claude-sdk/dist/runtime_check.js /opt/claude-sdk/dist/main.js \
    && node /opt/mcode-harness/check.mjs \
    && /opt/mcode-harness/native/cli.js --version \
    && mpirun --version
