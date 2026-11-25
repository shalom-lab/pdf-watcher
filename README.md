# pdf-watcher

<div align="center">

<img src="logo.svg" alt="pdf-watcher logo" width="128" />

**盯住文件夹里的 PDF / DOCX，安静收进 Zotero 10**

[![Platform](https://img.shields.io/badge/platform-Windows-0078D4?logo=windows&logoColor=white)](https://github.com/shalom-lab/pdf-watcher)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Zotero](https://img.shields.io/badge/Zotero-10%20Local%20API-CC2936)](https://www.zotero.org/)

</div>

## ✨ 功能特性

### 📂 监听与入库
- **仅本层目录**：托盘「添加目录」，不扫子目录
- **写稳再处理**：等坚果云 / 下载写完再抽文本
- **自动文集**：条目进 `pdf-watcher / 文件夹名`
- **同文献补附件**：按 DOI / PMID / 标题挂到已有条目

### 📚 题录来源
- **真源**：Crossref + PubMed（可双核查）
- **AI 只找 ID**：OpenAI 兼容接口（DeepSeek / OpenAI / 通义等）

### 🖥️ 托盘体验
- **单文件 exe**：图标与指南已嵌入
- **状态栏**：`Zotero：已连接` / `Zotero：不可用`
- **短菜单**：添加目录、监听目录、打开配置、打开日志…
- **中英界面**、配置热加载

## 🚀 快速开始

1. 安装并打开 **Zotero 10+**
2. **设置 → 高级**：勾选允许本机其他应用与 Zotero 通信
3. 运行 `pdf-watcher.exe`，在托盘（任务栏右下 ^）找到图标
4. 首次写入时选 **Always Allow**
5. 托盘 → **添加目录** → 放入 PDF / DOCX

> 没开 Zotero 也照常**监听**：新文件先进待处理队列，连上后自动入库。菜单显示 `Zotero：不可用` / `Zotero：已连接`。  
> 只关窗口、托盘里 Zotero 还在时接口仍可用；需托盘 **退出 Zotero** 才会变不可用。

## ⚙️ 配置

均在 `%APPDATA%\pdf-watcher\`（不写 exe 旁边）：

| 内容 | 文件 |
|------|------|
| 配置 | `config.yaml` |
| 日志 | `logs\` |
| 索引 | `index.db` |

AI（可选，OpenAI 兼容）：

```yaml
ai:
  enable: true
  apiKey: ""
  apiUrl: https://api.deepseek.com/v1/chat/completions
  model: deepseek-flash
  timeoutSec: 30
```

细节：托盘 → **帮助** → 中文指南 / 英文指南。

## 🛠️ 编译

```bash
go build -ldflags="-H=windowsgui -s -w" -o pdf-watcher.exe .
```

发布物即 **单文件 exe**。

## ⚠️ 注意

- **不要手改** `collectionId` / `collectionName`
- **不要轻易删** `index.db`：文献还在 Zotero，但后补附件可能挂不到旧条目
- 上传失败会删空壳附件，把文件再丢进监控目录即可

---

<div align="center">

**如果这个项目对您有帮助，请给个 ⭐ Star！**

Made with ❤️ by [shalom-lab](https://github.com/shalom-lab)

</div>
