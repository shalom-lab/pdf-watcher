# pdf-watcher User Guide

Watch a local folder (e.g. Nutstore sync) for new **PDF / DOCX** files, resolve bibliographic metadata via Crossref / PubMed, then create Zotero 10 items and upload attachments through the **Local API**.

## Before you start

1. Install and open **Zotero 10+**
2. **Settings → Advanced**: enable “Allow other applications on this computer to communicate with Zotero”
3. Run `pdf-watcher.exe`, find the tray icon (system tray / overflow ^), **right-click** for the menu
4. On first write, choose **Always Allow** in the Zotero dialog (not just Allow)

## Everyday use

1. Tray → **Add watch folder** (top-level only; subfolders are not watched)
2. Drop or sync PDF/DOCX into that folder
3. Wait until the file finishes writing; the app creates the item and attachment
4. Collections: **pdf-watcher → folder name** (if two watches share a basename, names become `parent/folder`)

## Where metadata comes from

- **Source of truth**: Crossref / PubMed (optional dual check when both DOI and PubMed hits exist)
- **LLM (optional)**: only helps discover DOI/PMID when regex fails — never used as bibliographic truth
- Configure `ai` (OpenAI-compatible): set `ai.apiKey` and `ai.enable: true`; point `apiUrl` at any `/v1/chat/completions` endpoint (DeepSeek / OpenAI / etc.)

## Config and data locations

Everything under the user profile (not next to the exe):

| What | Path |
|------|------|
| Config | `%APPDATA%\pdf-watcher\config.yaml` |
| Logs | `%APPDATA%\pdf-watcher\logs\` |
| Ingest index | `%APPDATA%\pdf-watcher\index.db` |

- Config is **hot-reloaded**
- Do **not** hand-edit `collectionId` / `collectionName`
- Do **not** delete `index.db` casually — Zotero items remain, but later attachments may not attach to prior items

## Attachments days later

If the index already has the same DOI/PMID (or a long normalized title), a new file is attached under the **existing parent item**. The app only remembers items **it successfully ingested**.

## FAQ

**Zotero not installed / not running**  
The tray app still starts (no crash). The menu shows “Zotero: unavailable” and the tooltip explains it; once Zotero 10 is open with local API enabled, status flips to connected and queued files are processed.

**No tray icon**  
Check the overflow area (^), or kill old processes and relaunch.

**403 / local API disabled**  
Enable local API in Zotero Advanced settings.

**need Always Allow**  
Grant Always Allow; `zotero.apiKey` will appear in config.

**Attachment missing in Zotero**  
Likely an incomplete upload shell — delete the item and drop the file again.

**Export Zotero collections**  
Exports the collection list (path/name/key) and opens it. Daily use still relies on auto `pdf-watcher/folder`; you usually do not need this.

## Build

```text
go build -ldflags="-H=windowsgui -s -w" -o pdf-watcher.exe .
```
