# syntax=docker/dockerfile:1
#
# 运行镜像（不在镜像里编译）：只装常用工具，放入**预编译好的 Linux 单二进制**。
# 二进制由 CI 的 binaries job 交叉编译（纯 Go、无 QEMU），按目标架构放在
# 构建上下文的 dist/<TARGETARCH>/artex。这样多架构构建时 arm64 只需模拟 apt 层，
# 不再模拟 Next/Go 编译，速度快得多。
#
# 本地手动构建镜像时，先自行准备二进制：
#   cd web && npm run build:static && cd ..
#   cp -r web/out server/webui/dist
#   CGO_ENABLED=0 GOARCH=amd64 go build -tags embedui -o dist/amd64/artex ./cmd/artex
#   docker build -t artex:local .
FROM python:3.12-slim-bookworm
ARG TARGETARCH
# 常用工具：ripgrep / curl / vim，加一批 recon 常备件（按需增删）。
# Node 从 NodeSource 装 22.x，与前端构建使用的主版本一致。
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates ripgrep curl wget vim git jq unzip \
      tzdata \
      dnsutils iputils-ping netcat-openbsd inetutils-telnet whois nmap \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*
# 工具箱预置工具（server/toolpresets.json）所需的二进制。全部固定版本、按 TARGETARCH
# 从 GitHub Releases 取 linux 构建；版本号与 toolpresets 的 version_cmd 对齐验证过。
# waybackurls 上游只发 linux amd64，arm64 构建跳过（卡片会显示缺失）。
# nuclei 模板随镜像预置到 /root/nuclei-templates，避免容器首次运行联网拉取。
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) pd=amd64; ferox=x86_64 ;; \
      arm64) pd=arm64; ferox=aarch64 ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    tmp="$(mktemp -d)"; \
    dl() { curl -fsSL --retry 3 --retry-delay 2 -o "$1" "$2"; }; \
    pdzip() { \
      name="$1"; ver="$2"; \
      dl "$tmp/$name.zip" "https://github.com/projectdiscovery/$name/releases/download/v$ver/${name}_${ver}_linux_${pd}.zip"; \
      unzip -q "$tmp/$name.zip" -d "$tmp/$name"; \
      install -m 0755 "$tmp/$name/$name" /usr/local/bin/; \
    }; \
    pdzip httpx 1.6.10; \
    pdzip subfinder 2.6.7; \
    pdzip dnsx 1.2.1; \
    pdzip tlsx 1.1.7; \
    pdzip naabu 2.7.0; \
    pdzip katana 1.1.2; \
    pdzip nuclei 3.3.5; \
    dl "$tmp/gau.tar.gz" "https://github.com/lc/gau/releases/download/v2.2.4/gau_2.2.4_linux_${pd}.tar.gz"; \
    tar -xzf "$tmp/gau.tar.gz" -C "$tmp" gau; \
    install -m 0755 "$tmp/gau" /usr/local/bin/gau; \
    dl "$tmp/ffuf.tar.gz" "https://github.com/ffuf/ffuf/releases/download/v2.3.0/ffuf_2.3.0_linux_${pd}.tar.gz"; \
    tar -xzf "$tmp/ffuf.tar.gz" -C "$tmp" ffuf; \
    install -m 0755 "$tmp/ffuf" /usr/local/bin/ffuf; \
    dl "$tmp/ferox.zip" "https://github.com/epi052/feroxbuster/releases/download/v2.14.0/${ferox}-linux-feroxbuster.zip"; \
    unzip -q "$tmp/ferox.zip" -d "$tmp/ferox"; \
    install -m 0755 "$tmp/ferox/feroxbuster" /usr/local/bin/feroxbuster; \
    if [ "$pd" = amd64 ]; then \
      dl "$tmp/wb.tgz" "https://github.com/tomnomnom/waybackurls/releases/download/v0.1.0/waybackurls-linux-amd64-0.1.0.tgz"; \
      tar -xzf "$tmp/wb.tgz" -C "$tmp" waybackurls; \
      install -m 0755 "$tmp/waybackurls" /usr/local/bin/waybackurls; \
    fi; \
    dl "$tmp/templates.tar.gz" "https://codeload.github.com/projectdiscovery/nuclei-templates/tar.gz/refs/tags/v10.0.2"; \
    tar -xzf "$tmp/templates.tar.gz" -C /root; \
    mv /root/nuclei-templates-10.0.2 /root/nuclei-templates; \
    mkdir -p /usr/share/wordlists/dirb; \
    dl /usr/share/wordlists/dirb/common.txt "https://raw.githubusercontent.com/danielmiessler/SecLists/master/Discovery/Web-Content/common.txt"; \
    rm -rf "$tmp"
# 预装 Playwright MCP 与 CLI（全局），运行时不再 npx 联网下载。
# @playwright/mcp：browser MCP 直接 `npx @playwright/mcp`（已全局装好，无需 -y/@latest）。
# @playwright/cli：提供 playwright-cli，装完顺带 --help 验证可执行。
# 再装 playwright（提供浏览器管理），装完用 --with-deps 预置 chromium 及其系统依赖，
# 这样容器内 MCP/CLI 首次启动即可用，不再联网下载浏览器。
RUN npm install -g @playwright/mcp@latest @playwright/cli@latest playwright@latest \
    && playwright-cli --help \
    && playwright install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/*
# red-skill 的 check/update 需要 PyYAML；find/show 不依赖，但固化避免容器重建后丢失。
RUN python3 -m pip install --no-cache-dir pyyaml
WORKDIR /app
ENV TZ=Asia/Shanghai
# JWT 签名密钥目录。必须挂持久卷：放在 /app（容器层）里每次重建容器都会换 key，
# 所有人被登出。compose 把 ./keys 绑到这里；单独 docker run 时 VOLUME 也会兜底。
ENV ARTEX_KEYS_DIR=/app/keys
# 预编译好的对应架构二进制（dist/amd64/artex 或 dist/arm64/artex）
COPY dist/${TARGETARCH}/artex /app/artex
# 守护启动脚本：进程退出后按退出码决定是否重新拉起，页面一键更新靠它完成换装。
# 它同时负责把 SIGTERM 转发给 artex —— docker stop 只把信号发给 PID 1，
# 不转发的话 artex 收不到、做不了优雅关闭，10 秒后被 SIGKILL 硬杀。
COPY start.sh /app/start.sh
RUN chmod +x /app/artex /app/start.sh
COPY skills/ /app/skills/
# data/（SQLite）与 keys/（jwt.key）持久化点
VOLUME ["/app/data", "/app/keys"]
EXPOSE 8787 8788
ENTRYPOINT ["/app/start.sh"]
CMD ["-addr", ":8787", "-proxy", ":8788"]
