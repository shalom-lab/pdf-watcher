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
- **仅本层目录**：托盘添加监控文件夹，不扫子目录
- **稳定写入后再处理**：等坚果云 / 下载写完再抽文本
- **自动文集**：条目进 `pdf-watcher / 文件夹名`
- **同文献补附件**：按 DOI / PMID / 标题索引挂到已有条目

### 📚 题录来源
- **真源**：Crossref + PubMed（可双核查）
- **AI 只找 ID**：OpenAI 兼容接口（DeepSeek / OpenAI / 通义等），不直接当题录

### 🖥️ 托盘体验
- **单文件 exe**：图标与指南已嵌入，无额外依赖
- **Zotero 状态**：未安装 / 未打开时显示「不可用」，连上后自动恢复
- **中英界面**与配置热加载

## 🚀 快速开始

1. 安装并打开 **Zotero 10+**
2. **设置 → 高级**：勾选允许本机其他应用与 Zotero 通信
3. 运行 `pdf-watcher.exe`，在托盘（任务栏右下 ^）找到图标
4. 首次写入时选 **Always Allow**
5. 托盘 → **添加监控文件夹** → 放入 PDF / DOCX

> 没开 Zotero 也能启动托盘，不会崩溃；菜单会标「不可用」，打开后自动变为已连接。  
> 只关窗口、托盘里 Zotero 还在时，本机 API 仍可用，显示「已连接」是正常的；要完全退出 Zotero 才会变不可用。

## ⚙️ 配置

路径均在 `%APPDATA%\pdf-watcher\`（不写 exe 旁边）：

| 内容 | 文件 |
|------|------|
| 配置 | `config.yaml` |
| 日志 | `logs\` |
| 入库索引 | `index.db` |

AI（可选，OpenAI 兼容）：

```yaml
ai:
  enable: true
  apiKey: ""
  apiUrl: https://api.deepseek.com/v1/chat/completions
  model: deepseek-chat
  timeoutSec: 30
```

更细的说明：托盘 → **帮助** → 中文 / English 指南。

## 🛠️ 编译

```bash
go build -ldflags="-H=windowsgui -s -w" -o pdf-watcher.exe .
```

发布物就是这一份 **单文件 exe**。

## ⚠️ 注意

- **不要手改** `collectionId` / `collectionName`（程序维护）
- **不要轻易删** `index.db`：不会丢 Zotero 文献，但后补附件可能挂不到旧条目
- 附件上传失败时会删掉空壳，把文件再丢进监控目录即可

---

<div align="center">

**如果这个项目对您有帮助，请给个 ⭐ Star！**

Made with ❤️ by [shalom-lab](https://github.com/shalom-lab)

</div>
