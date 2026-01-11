<div align="center">
    <h1>🔮 MoMorph CLI</h1>
    <h3><em>Initialize MoMorph projects for AI-driven development</em></h3>
</div>

<p align="center">
    <strong>A command-line tool for initializing MoMorph projects during development. MoMorph CLI helps developers set up design-driven AI development environments by connecting UI designs with specifications, tests, and implementation through AI agent collaboration.</strong>
</p>

<p align="center">
    <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go" alt="Go Version"/></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"/></a>
</p>

---

## Table of Contents

- [🤔 What is MoMorph CLI?](#-what-is-momorph-cli)
- [⚡ Get Started](#-get-started)
- [🤖 Supported AI Agents](#-supported-ai-agents)
- [🔧 MoMorph CLI Reference](#-mm-cli-reference)
- [💻 Development](#-development)
- [📄 License](#-license)

## 🤔 What is MoMorph CLI?

**MoMorph** is an enterprise software development collaboration platform that leverages AI to connect the entire development process, with UI design as the central axis. It seamlessly integrates with AI agents to automate the loop of "Design → AI → Specifications → Tests → Implementation" in a flexible and continuous manner.

**MoMorph CLI** is the command-line tool that initializes MoMorph projects during the development phase. It sets up the necessary project structure, configurations, and workflows to enable design-driven AI development, helping teams eliminate information gaps and standardize development processes.

### About MoMorph Platform

MoMorph enables:
- **Design-Driven Development**: Start from UI designs in Figma and automatically generate specifications, test cases, and code
- **AI-Powered Collaboration**: Seamlessly connect with AI coding assistants (GitHub Copilot, Cursor, Claude Code) throughout the development lifecycle
- **Comprehensive Integration**: From specification generation to test case creation, code generation, and GitHub integration - all supported by AI
- **Multilingual Support**: Suitable for multinational team development environments

### Key Benefits of MoMorph CLI

- **Standardized Setup**: Initialize projects with MoMorph's design-driven development structure
- **AI-Ready Configuration**: Pre-configure prompts, rules, and agent modes for optimal AI collaboration
- **Quick Initialization**: Set up complete MoMorph development environments in seconds
- **Multi-Platform**: Works on macOS, Linux, and Windows
- **Version Control**: Keep configurations synchronized with the latest MoMorph templates

## ⚡ Get Started

### 1. Install MoMorph CLI

Choose your preferred installation method:

#### Option 1: Homebrew (macOS/Linux)

```bash
brew install momorph/tap/momorph-cli
```

Then use the tool directly:

```bash
# Login with GitHub
momorph login

# Initialize new MoMorph project
momorph init <PROJECT_NAME> --ai copilot

# Or initialize in existing project
momorph init . --ai cursor
```

#### Option 2: Chocolatey (Windows)

```bash
choco install momorph
```

#### Option 3: Shell Script (Linux/macOS)

```bash
curl -fsSL https://momorph.ai/cli/stable/install.sh | bash
```

#### Option 4: Go Install

```bash
go install github.com/momorph/cli@latest
```

**Benefits of package manager installation:**
- Tool stays installed and available in PATH
- Easy updates with package manager commands
- Better dependency management
- Cleaner system configuration

#### Verify setup

Check your account information and configuration:

```bash
# Display current account info
momorph whoami

# Check CLI version
momorph version
```

### 2. Authenticate with GitHub

Launch MoMorph CLI and authenticate using GitHub OAuth Device Flow:

```bash
momorph login
```

The CLI will display a user code and authentication link. Open the link in your browser and enter the code to complete authentication.

### 3. Initialize your MoMorph project

Use the `momorph init` command to set up a MoMorph project with design-driven AI development workflow:

```bash
# Initialize in a new directory
momorph init viblo --ai copilot

# Initialize in current directory
momorph init . --ai cursor

# Initialize without confirmation (force mode)
momorph init . --force --ai claude
```

The CLI will:
- Download the latest MoMorph project template from backend
- Extract configuration files (`.claude`, `.github`, prompt files, workflow scripts, etc.)
- Set up the project structure for design-driven AI development
- Configure AI agent integration for the specified assistant (Copilot, Cursor, or Claude Code)

### 4. Environment Configuration (Optional)

By default, MoMorph CLI connects to the production MoMorph API. For development or testing purposes, you can configure the CLI using environment variables.

#### Using Custom API Endpoint

```bash
# Use a custom API endpoint
export MOMORPH_API_ENDPOINT=https://custom.momorph.com

# Commands will now use the custom endpoint
momorph login
momorph init my-project --ai copilot
```

#### Available Environment Variables

| Environment Variable          | Description                       | Required | Example Value            |
| ----------------------------- | --------------------------------- | -------- | ------------------------ |
| `MOMORPH_API_ENDPOINT`        | Custom API endpoint URL           | No       | `https://momorph.ai`     |
| `MOMORPH_MCP_ENDPOINT`        | Custom MCP endpoint URL           | No       | `https://momorph.ai/mcp` |
| `MOMORPH_BASIC_AUTH_USERNAME` | Basic Auth username (if required) | No       | `your_username`          |
| `MOMORPH_BASIC_AUTH_PASSWORD` | Basic Auth password (if required) | No       | `your_password`          |

**Environment Priority**:
1. `MOMORPH_API_ENDPOINT` (highest priority for API)
2. `MOMORPH_MCP_ENDPOINT` (highest priority for MCP)
3. Default production URLs (lowest priority)

## 🤖 Supported AI Agents

| Agent                                                 | Support | Notes                            |
| ----------------------------------------------------- | ------- | -------------------------------- |
| [GitHub Copilot](https://github.com/features/copilot) | ✅       | Full support with custom prompts |
| [Cursor](https://cursor.sh/)                          | 🔜       | Coming soon                      |
| [Claude Code](https://www.anthropic.com/claude-code)  | 🔜       | Coming soon                      |

> Currently, only GitHub Copilot is fully supported. Support for Cursor and Claude Code is coming soon. If you encounter issues, please [open an issue](https://github.com/momorph/cli/issues/new).

## 🔧 MoMorph CLI Reference

The `momorph` command supports the following operations:

### Commands

| Command   | Description                                                 |
| --------- | ----------------------------------------------------------- |
| `login`   | Authenticate with GitHub using OAuth Device Flow            |
| `init`    | Initialize a MoMorph project with AI agent configurations   |
| `whoami`  | Display current account information and subscription status |
| `update`  | Update MoMorph CLI to the latest version                    |
| `version` | Show MoMorph CLI version information                        |
| `help`    | Display help information                                    |

### Examples

```bash
# Basic project initialization
momorph init my-project

# Initialize with specific AI assistant
momorph init my-project --ai copilot
momorph init my-project --ai cursor
momorph init my-project --ai claude

# Initialize in current directory
momorph init . --ai copilot

# Force initialization without confirmation
momorph init . --force --ai cursor

# Check account information
momorph whoami

# Update CLI
momorph update

# Check version
momorph version
```

## 💻 Development

### Setup

```bash
# Clone repository
git clone https://github.com/momorph/cli.git
cd momorph-cli

# Install dependencies
go mod download

# Build
go run main.go -- <command>
```

### Project Structure

```
momorph-cli/
├── cmd/                 # CLI commands
│   ├── login.go
│   ├── init.go
│   ├── whoami.go
│   ├── update.go
│   ├── version.go
│   └── root.go
├── internal/            # Internal packages
│   ├── auth/           # Authentication logic
│   ├── api/            # API client
│   ├── config/         # Configuration management
│   ├── template/       # Template management
│   └── ui/             # UI components
├── pkg/                # Public packages
├── go.mod
├── go.sum
├── main.go
└── README.md
```

### Running Tests

```bash
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

### Building for Release

```bash
# Create snapshot release
goreleaser release --snapshot --clean

# Build for all platforms
goreleaser build --snapshot --clean
```

## 📄 License

This project is licensed under the terms of the MIT open source license. Please refer to the [LICENSE](LICENSE) file for the full terms.

---

<div align="center">Made with ❤️ by the MoMorph Team</div>
