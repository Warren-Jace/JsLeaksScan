# JsLeaksScan

JsLeaksScan 是一个用于扫描 JavaScript 文件（以及其他文本文件）以查找潜在敏感信息（如 API 密钥、密码、内部端点等）的工具。它支持扫描本地文件系统中的文件和在线 URL。扫描规则是可配置的，允许使用正则表达式和字面量字符串进行匹配。

## 功能特性

*   **本地扫描**: 递归扫描指定目录下的文件。
*   **URL 扫描**: 扫描单个 URL 或从文件加载的 URL 列表。
*   **可配置规则**: 通过 JSON 文件定义扫描规则，支持：
    *   **正则表达式**: 用于复杂的模式匹配。
    *   **字面量字符串**: 用于快速查找精确的文本片段。
*   **并发扫描**: 利用 Go 的并发特性提高扫描速度，尤其是在处理大量文件或 URL 时。
*   **灵活的 HTTP 选项 (URL 扫描)**:
    *   支持 HTTP/HTTPS 代理。
    *   自定义请求头 (Headers)。
    *   指定请求方法 (GET, POST 等)。
    *   发送 POST 请求数据 (Body)。
    *   设置 Cookies。
    *   设置 Referer。
    *   设置 User-Agent。
    *   HTTP Basic Authentication 认证。
    *   可配置的请求超时时间。
*   **结果输出**: 将发现的匹配项保存到指定的输出目录中，每个源文件或 URL 对应一个结果文件。
*   **输出控制**: 提供详细模式 (`-v`) 和静默模式 (`-q`) 来控制程序输出。

## 安装

你需要安装 Go 环境 (建议版本 1.18 或更高)。

1.  **获取代码**:
    ```bash
    git clone <your-repository-url> # 替换为你的仓库 URL
    cd jsleaksscan
    ```
    或者直接下载源代码压缩包并解压。

2.  **编译**:
    在项目根目录 (`jsleaksscan`) 下运行：
    ```bash
    go build -o jsleaksscan ./cmd/jsleaksscan/
    ```
    这将在当前目录下生成一个名为 `jsleaksscan` (Linux/macOS) 或 `jsleaksscan.exe` (Windows) 的可执行文件。
    

## 使用方法

```bash
jsleaksscan <mode> [options]
```

### 模式 (Mode)

*   `localScan`: 启用本地文件扫描模式。
*   `urlScan`: 启用在线 URL 扫描模式。

### 基本选项 (适用于所有模式)

*   `-h`, `--help`: 显示帮助信息。可以与模式结合使用（例如 `jsleaksscan localScan -h`）查看特定模式的帮助。
*   `-c <file>`: 指定规则配置文件的路径 (默认: `config.json`)。
*   `-od <dir>`, `--outputDir <dir>`: 指定结果输出目录 (默认: `results`)。
*   `-t <num>`: 设置并发数。
    *   在 `localScan` 模式下，控制并发处理文件的数量 (默认: CPU 核心数 * 2)。
    *   在 `urlScan` 模式下，控制并发请求 URL 的数量 (默认: 50)。
*   `-v`, `--verbose`: 启用详细输出，显示更多过程信息。
*   `-q`, `--quiet`: 启用静默模式，只输出错误和最终的匹配结果文件信息（覆盖 `-v`）。

### `localScan` 模式选项

*   `-d <dir>`, `--dirname <dir>`: **必需**。指定包含要扫描文件的本地目录路径。

### `urlScan` 模式选项

*   `-u <url>`, `--url <url>`: 指定要扫描的单个 URL。
*   `-uf <file>`, `--urlFileName <file>`: 指定包含要扫描 URL 列表的文件路径。
    *   **注意**: `-u` 和 `-uf` 必须提供一个且只能提供一个。
*   `-p <proxy>`, `--proxy <proxy>`: 设置 HTTP/HTTPS 代理 (例如: `http://127.0.0.1:8080`, `socks5://user:pass@host:port`)。
*   `-H <header>`, `--header <header>`: 设置自定义 HTTP 请求头。
    *   格式 1: `"Key: Value"`
    *   格式 2: `"Key1:Value1,Key2:Value2"`
    *   格式 3 (JSON): `'{"Key1":"Value1", "Key2":"Value2"}'` (注意在 shell 中可能需要用单引号包裹 JSON 字符串)
*   `-m <method>`, `--method <method>`: 指定 HTTP 请求方法 (默认: `GET`)。
*   `--data <data>`: 指定 POST 请求的 body 数据。
*   `--cookie <cookie>`: 设置 HTTP Cookie。
*   `-r <referer>`, `--referer <referer>`: 设置 HTTP Referer。
*   `-ua <agent>`, `--userAgent <agent>`: 设置 HTTP User-Agent。
*   `-a <auth>`, `--auth <auth>`: 设置 HTTP Basic Authentication 凭证 (格式: `username:password`)。
*   `--timeout <seconds>`: 设置请求超时时间 (单位: 秒, 默认: 10)。

## 配置文件 (`config.json`)

配置文件是一个 JSON 对象，其中：

*   **键 (Key)**: 是你为规则命的自定义名称（例如 `aws_key`, `google_api`, `internal_api`）。
*   **值 (Value)**: 是用于匹配的模式字符串。
    *   如果字符串不包含正则表达式元字符，它将被视为**字面量**进行快速匹配。
    *   如果字符串包含正则表达式元字符，它将被编译为**正则表达式**进行匹配。

**示例 `config.json`**:

```json
{
  "google_api_key": "AIza[0-9A-Za-z\\-_]{35}",
  "aws_access_key_id": "AKIA[0-9A-Z]{16}",
  "slack_token": "(xox[pboa]|xoxr|xapp)-[0-9a-zA-Z]{10,48}",
  "ssh_private_key": "-----BEGIN ((EC|PGP|DSA|RSA|OPENSSH) )?PRIVATE KEY-----",
  "possible_internal_api": "https?://api\\.internal\\.[a-zA-Z0-9./-]+",
  "hardcoded_password": "password: \"test1234\"",
  "debug_endpoint": "/_debug/pprof"
}
```

## 示例

1.  **扫描本地目录 `~/projects/my-app/js`**:
    ```bash
    ./jsleaksscan localScan -d ~/projects/my-app/js -c config.json -od my_app_results -t 16
    ```

2.  **扫描 `urls.txt` 文件中的所有 URL，使用 100 个并发线程**:
    ```bash
    ./jsleaksscan urlScan -uf urls.txt -c config.json -t 100 -od url_scan_results
    ```

3.  **扫描单个 URL**:
    ```bash
    ./jsleaksscan urlScan -u https://example.com/assets/main.js -c config.json
    ```

4.  **扫描 URL 列表，并使用 HTTP 代理**:
    ```bash
    ./jsleaksscan urlScan -uf sensitive_urls.txt -c config.json -p http://127.0.0.1:8080
    ```

5.  **扫描单个 URL，使用 POST 方法并发送数据，同时设置自定义 Header 和 Cookie**:
    ```bash
    ./jsleaksscan urlScan -u https://api.example.com/data -m POST --data '{"param":"value"}' -H 'Content-Type: application/json' --cookie 'sessionid=xyzabc' -c config.json
    ```

6.  **扫描本地目录，并启用详细输出**:
    ```bash
    ./jsleaksscan localScan -d /path/to/scan -v
    ```
