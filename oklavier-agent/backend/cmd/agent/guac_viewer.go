package main

import (
	"encoding/json"
	htmlpkg "html"
	"sort"
	"strings"
)

func guacViewerHTML(sessionID, agentName, controlPlaneURL, protocol, lang, defaultSettings string, shadow bool) string {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
<title>Oklavier - Server Session</title>
<style>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap');
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #000; overflow: hidden; font-family: 'Inter', system-ui, sans-serif; color: rgba(255,255,255,0.9); }

/* Display */
#display { width: 100vw; height: 100vh; background: #0f1225; position: relative; overflow: hidden; }
#display > div { position: absolute; top: 0; left: 0; transform-origin: top left; }
#display canvas { cursor: none; }

/* Overlay — connecting */
#overlay { position: fixed; inset: 0; z-index: 100; background: #0f1225; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 16px; transition: opacity 0.5s; }
#overlay.hidden { opacity: 0; pointer-events: none; }
.spinner { width: 56px; height: 56px; border: 3px solid rgba(112,150,255,0.15); border-top-color: #7096ff; border-radius: 50%; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
#status { color: rgba(255,255,255,0.4); font-size: 13px; }

/* Overlay — disconnected */
#disconnect-overlay { display: none; position: fixed; inset: 0; z-index: 150; background: rgba(15,18,37,0.85); backdrop-filter: blur(8px); align-items: center; justify-content: center; }
#disconnect-overlay.show { display: flex; }
#disconnect-box { text-align: center; max-width: 380px; }
#disconnect-box .dc-icon { width: 64px; height: 64px; margin: 0 auto 20px; border-radius: 50%; background: rgba(239,68,68,0.1); display: flex; align-items: center; justify-content: center; }
#disconnect-box .dc-icon svg { width: 28px; height: 28px; stroke: #ef4444; }
#disconnect-box h2 { font-size: 20px; font-weight: 700; color: rgba(255,255,255,0.9); margin-bottom: 8px; }
#disconnect-box p { font-size: 13px; color: rgba(255,255,255,0.35); margin-bottom: 28px; line-height: 1.5; }
#disconnect-box .dc-btns { display: flex; flex-direction: column; gap: 8px; }
#disconnect-box .dc-btn { padding: 12px; border-radius: 10px; font-size: 13px; font-weight: 600; font-family: inherit; cursor: pointer; border: none; transition: all 0.15s; text-align: center; text-decoration: none; display: block; }
#disconnect-box .dc-btn.primary { background: linear-gradient(135deg, #7096ff, #65d5c5); color: #fff; }
#disconnect-box .dc-btn.primary:hover { opacity: 0.9; transform: translateY(-1px); }
#disconnect-box .dc-btn.secondary { background: rgba(255,255,255,0.06); color: rgba(255,255,255,0.6); border: 1px solid rgba(255,255,255,0.08); }
#disconnect-box .dc-btn.secondary:hover { background: rgba(255,255,255,0.1); color: rgba(255,255,255,0.9); }
#disconnect-box .dc-btn.danger { background: rgba(239,68,68,0.1); color: rgba(239,68,68,0.7); border: 1px solid rgba(239,68,68,0.15); }
#disconnect-box .dc-btn.danger:hover { background: rgba(239,68,68,0.2); color: #ef4444; }

/* Side tab — draggable pill on left edge */
#side-tab { position: fixed; left: 0; z-index: 50; width: 22px; height: 56px; background: rgba(15,18,37,0.7); backdrop-filter: blur(16px); border: 1px solid rgba(255,255,255,0.12); border-left: none; border-radius: 0 10px 10px 0; cursor: pointer; display: flex; align-items: center; justify-content: center; opacity: 0.5; transition: opacity 0.3s, left 0.3s, background 0.2s; box-shadow: 2px 2px 12px rgba(0,0,0,0.3); }
#side-tab:hover { opacity: 1; background: rgba(15,18,37,0.85); }
#side-tab svg { width: 10px; height: 10px; fill: rgba(255,255,255,0.6); }
body.sb-open #side-tab { left: 260px; opacity: 1; }
body.sb-open #side-tab svg { transform: rotate(180deg); }

/* Sidebar — glassmorphism panel */
#sidebar { position: fixed; left: 0; top: 0; bottom: 0; z-index: 40; width: 260px; background: rgba(15,18,37,0.65); backdrop-filter: blur(40px) saturate(1.5); -webkit-backdrop-filter: blur(40px) saturate(1.5); border-right: 1px solid rgba(255,255,255,0.08); transform: translateX(-100%); transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1); display: flex; flex-direction: column; }
body.sb-open #sidebar { transform: translateX(0); }
.sb-hdr { padding: 18px 16px; display: flex; align-items: center; gap: 10px; border-bottom: 1px solid rgba(255,255,255,0.06); }
.sb-logo-svg { width: 28px; height: 28px; flex-shrink: 0; }
.sb-logo-text { font-size: 1.15rem; font-weight: 800; letter-spacing: -0.02em; background: linear-gradient(135deg, #7096ff, #65d5c5); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
.sb-x { background: none; border: none; color: rgba(255,255,255,0.25); cursor: pointer; font-size: 20px; line-height: 1; transition: color 0.15s; }
.sb-x:hover { color: rgba(255,255,255,0.6); }
.sb-scroll { flex: 1; overflow-y: auto; overflow-x: hidden; }
.sb-scroll::-webkit-scrollbar { width: 4px; }
.sb-scroll::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 2px; }
.sb-menu { padding: 8px 12px; display: flex; flex-direction: column; gap: 4px; }
.sb-btn { display: flex; align-items: center; gap: 12px; padding: 12px 14px; color: rgba(255,255,255,0.5); font-size: 13px; font-weight: 500; cursor: pointer; transition: all 0.15s; border-radius: 10px; border: 1px solid transparent; }
.sb-btn:hover { background: rgba(255,255,255,0.06); color: rgba(255,255,255,0.9); border-color: rgba(255,255,255,0.06); }
.sb-btn svg { width: 18px; height: 18px; flex-shrink: 0; opacity: 0.6; }
.sb-btn:hover svg { opacity: 1; }
.sb-btn.destroy { color: rgba(239,68,68,0.6); }
.sb-btn.destroy:hover { background: rgba(239,68,68,0.08); color: #ef4444; border-color: rgba(239,68,68,0.15); }
.sb-btn.destroy:hover svg { opacity: 1; }
.sb-ft { padding: 16px 12px; border-top: 1px solid rgba(255,255,255,0.06); display: flex; flex-direction: column; gap: 4px; }
.sb-info { padding: 12px 14px; font-size: 11px; color: rgba(255,255,255,0.2); line-height: 1.6; }
.sb-section { padding: 4px 12px; }
.sb-section-title { font-size: 10px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: rgba(255,255,255,0.2); padding: 8px 14px 4px; }
.sb-toggle { display: flex; align-items: center; justify-content: space-between; padding: 7px 14px; border-radius: 8px; cursor: pointer; transition: background 0.1s; }
.sb-toggle:hover { background: rgba(255,255,255,0.04); }
.sb-toggle-label { font-size: 12px; color: rgba(255,255,255,0.45); font-weight: 500; }
.sb-toggle-switch { position: relative; width: 32px; height: 18px; border-radius: 9px; background: rgba(255,255,255,0.12); transition: background 0.2s; flex-shrink: 0; }
.sb-toggle-switch.on { background: rgba(112,150,255,0.5); }
.sb-toggle-switch::after { content: ''; position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: rgba(255,255,255,0.5); transition: all 0.2s; }
.sb-toggle-switch.on::after { left: 16px; background: #fff; }
.sb-div { height: 1px; background: rgba(255,255,255,0.06); margin: 6px 12px; }
.sb-reconnecting { position: fixed; top: 12px; right: 12px; z-index: 200; background: rgba(112,150,255,0.15); backdrop-filter: blur(12px); border: 1px solid rgba(112,150,255,0.3); border-radius: 8px; padding: 8px 16px; font-size: 12px; color: rgba(112,150,255,0.9); display: none; }
.sb-reconnecting.show { display: block; }
.sb-select { padding: 7px 14px; }
.sb-select label { font-size: 10px; font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; color: rgba(255,255,255,0.2); display: block; margin-bottom: 4px; }
.sb-select select { width: 100%; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; color: rgba(255,255,255,0.7); font-size: 12px; font-family: inherit; padding: 6px 8px; appearance: none; -webkit-appearance: none; cursor: pointer; outline: none; transition: border-color 0.15s; background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='rgba(255,255,255,0.3)'/%3E%3C/svg%3E"); background-repeat: no-repeat; background-position: right 8px center; }
.sb-select select:hover { border-color: rgba(255,255,255,0.15); }
.sb-select select:focus { border-color: rgba(112,150,255,0.4); }
.sb-select select option { background: #1a1f36; color: rgba(255,255,255,0.8); }

/* Confirm modal */
#confirm-modal { display: none; position: fixed; inset: 0; z-index: 210; }
#confirm-modal.show { display: flex; align-items: center; justify-content: center; }
#confirm-modal .modal-backdrop { position: absolute; inset: 0; background: rgba(0,0,0,0.5); backdrop-filter: blur(4px); }
#confirm-modal .modal-box { position: relative; width: 360px; background: rgba(15,18,37,0.95); backdrop-filter: blur(40px); border: 1px solid rgba(255,255,255,0.1); border-radius: 16px; padding: 28px; text-align: center; }
#confirm-modal .modal-icon { width: 48px; height: 48px; margin: 0 auto 16px; border-radius: 50%; background: rgba(239,68,68,0.1); display: flex; align-items: center; justify-content: center; }
#confirm-modal .modal-icon svg { width: 24px; height: 24px; stroke: #ef4444; }
#confirm-modal .modal-title { font-size: 16px; font-weight: 700; color: rgba(255,255,255,0.9); margin-bottom: 8px; }
#confirm-modal .modal-msg { font-size: 13px; color: rgba(255,255,255,0.4); margin-bottom: 24px; line-height: 1.5; }
#confirm-modal .modal-btns { display: flex; gap: 10px; }
#confirm-modal .modal-btn { flex: 1; padding: 10px; border-radius: 10px; font-size: 13px; font-weight: 600; font-family: inherit; cursor: pointer; border: none; transition: all 0.15s; }
#confirm-modal .modal-btn.cancel { background: rgba(255,255,255,0.06); color: rgba(255,255,255,0.5); border: 1px solid rgba(255,255,255,0.08); }
#confirm-modal .modal-btn.cancel:hover { background: rgba(255,255,255,0.1); color: rgba(255,255,255,0.8); }
#confirm-modal .modal-btn.danger { background: rgba(239,68,68,0.15); color: #ef4444; border: 1px solid rgba(239,68,68,0.2); }
#confirm-modal .modal-btn.danger:hover { background: rgba(239,68,68,0.25); }

/* Upload modal */
#upload-modal { display: none; position: fixed; inset: 0; z-index: 200; }
#upload-modal.show { display: flex; align-items: center; justify-content: center; }
#upload-backdrop { position: absolute; inset: 0; background: rgba(0,0,0,0.5); backdrop-filter: blur(4px); }
#upload-box { position: relative; width: 420px; max-height: 80vh; background: rgba(15,18,37,0.95); backdrop-filter: blur(40px); border: 1px solid rgba(255,255,255,0.1); border-radius: 16px; padding: 24px; overflow-y: auto; }
#upload-box h3 { font-size: 16px; font-weight: 700; color: rgba(255,255,255,0.9); margin-bottom: 16px; }
#upload-close { position: absolute; top: 16px; right: 16px; background: none; border: none; color: rgba(255,255,255,0.3); font-size: 20px; cursor: pointer; }
#upload-close:hover { color: rgba(255,255,255,0.7); }
#upload-dropzone { border: 2px dashed rgba(112,150,255,0.25); border-radius: 12px; padding: 32px 16px; text-align: center; cursor: pointer; transition: all 0.2s; margin-bottom: 16px; }
#upload-dropzone:hover, #upload-dropzone.dragover { border-color: rgba(112,150,255,0.6); background: rgba(112,150,255,0.05); }
#upload-dropzone svg { width: 32px; height: 32px; stroke: rgba(112,150,255,0.4); margin: 0 auto 8px; display: block; }
#upload-dropzone p { font-size: 13px; color: rgba(255,255,255,0.35); }
#upload-dropzone span { color: rgba(112,150,255,0.8); text-decoration: underline; cursor: pointer; }
#upload-list { display: flex; flex-direction: column; gap: 8px; }
.upload-item { background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.06); border-radius: 10px; padding: 10px 14px; }
.upload-item-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.upload-item-name { font-size: 12px; color: rgba(255,255,255,0.7); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 260px; }
.upload-item-size { font-size: 10px; color: rgba(255,255,255,0.25); flex-shrink: 0; }
.upload-item-bar { height: 4px; background: rgba(255,255,255,0.08); border-radius: 2px; overflow: hidden; }
.upload-item-fill { height: 100%; background: linear-gradient(90deg, #7096ff, #65d5c5); border-radius: 2px; transition: width 0.15s; width: 0%; }
.upload-item.done .upload-item-fill { background: #10b981; width: 100% !important; }
.upload-item.done .upload-item-name { color: rgba(16,185,129,0.8); }
.upload-item.error .upload-item-fill { background: #ef4444; }

/* Peripherals modal */
#periph-modal { display: none; position: fixed; inset: 0; z-index: 200; }
#periph-modal.show { display: flex; align-items: center; justify-content: center; }
#periph-backdrop { position: absolute; inset: 0; background: rgba(0,0,0,0.5); backdrop-filter: blur(4px); }
/* Outer box does NOT scroll. Header is sticky-ish (just stays at top), the
   inner list scrolls so the scrollbar lives inside the rounded border, not
   along the outer edge. */
#periph-box { position: relative; width: 480px; max-height: 80vh; background: rgba(15,18,37,0.95); backdrop-filter: blur(40px); border: 1px solid rgba(255,255,255,0.1); border-radius: 16px; padding: 24px 24px 20px; overflow: hidden; display: flex; flex-direction: column; }
#periph-box h3 { font-size: 16px; font-weight: 700; color: rgba(255,255,255,0.9); margin-bottom: 16px; flex-shrink: 0; }
#periph-close { position: absolute; top: 16px; right: 16px; background: none; border: none; color: rgba(255,255,255,0.3); font-size: 20px; cursor: pointer; z-index: 1; }
#periph-close:hover { color: rgba(255,255,255,0.7); }
#periph-reload { position: absolute; top: 18px; right: 50px; background: none; border: none; color: rgba(255,255,255,0.3); cursor: pointer; padding: 2px; line-height: 0; z-index: 1; }
#periph-reload:hover { color: rgba(255,255,255,0.7); }
#periph-reload svg { width: 16px; height: 16px; }
#periph-list { display: flex; flex-direction: column; gap: 6px; overflow-y: auto; flex: 1; padding-right: 6px; margin-right: -6px; }
/* Custom thin scrollbar — Chromium */
#periph-list::-webkit-scrollbar { width: 6px; }
#periph-list::-webkit-scrollbar-track { background: transparent; }
#periph-list::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.12); border-radius: 6px; transition: background 0.2s; }
#periph-list::-webkit-scrollbar-thumb:hover { background: rgba(112,150,255,0.45); }
/* Firefox */
#periph-list { scrollbar-width: thin; scrollbar-color: rgba(255,255,255,0.12) transparent; }
.periph-item { display: flex; justify-content: space-between; align-items: center; gap: 12px; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.06); border-radius: 10px; padding: 10px 14px; }
.periph-label { font-size: 13px; color: rgba(255,255,255,0.8); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex: 1; }
.periph-toggle { position: relative; display: inline-block; width: 36px; height: 20px; flex-shrink: 0; }
.periph-toggle input { opacity: 0; width: 0; height: 0; }
.periph-toggle .periph-slider { position: absolute; cursor: pointer; inset: 0; background: rgba(255,255,255,0.15); transition: 0.2s; border-radius: 20px; }
.periph-toggle .periph-slider:before { position: absolute; content: ""; height: 14px; width: 14px; left: 3px; top: 3px; background: white; transition: 0.2s; border-radius: 50%; }
.periph-toggle input:checked + .periph-slider { background: linear-gradient(90deg, #7096ff, #65d5c5); }
.periph-toggle input:checked + .periph-slider:before { transform: translateX(16px); }
.periph-perm { background: rgba(112,150,255,0.08); border: 1px solid rgba(112,150,255,0.2); border-radius: 10px; padding: 16px; text-align: center; margin-bottom: 12px; }
.periph-perm p { font-size: 12px; color: rgba(255,255,255,0.6); margin-bottom: 12px; }
.periph-perm button { background: linear-gradient(90deg, #7096ff, #65d5c5); color: white; border: none; padding: 8px 16px; border-radius: 8px; cursor: pointer; font-size: 13px; font-weight: 600; }
/* Sticky footer (add row) — wraps multiple compact buttons (USB, BT,
   Serial, Folder, Screen, MIDI). Each is hidden when the matching API
   isn't available in the browser. */
#periph-add-row { flex-shrink: 0; padding-top: 12px; margin-top: 12px; border-top: 1px solid rgba(255,255,255,0.06); display: flex; flex-wrap: wrap; gap: 6px; }
#periph-add-row:empty { display: none; }
#periph-add-row button { background: rgba(255,255,255,0.04); color: rgba(255,255,255,0.7); border: 1px dashed rgba(255,255,255,0.12); padding: 8px 12px; border-radius: 8px; cursor: pointer; font-size: 11px; flex: 1 1 auto; min-width: 90px; white-space: nowrap; }
#periph-add-row button:hover { background: rgba(112,150,255,0.05); border-color: rgba(112,150,255,0.4); color: rgba(255,255,255,0.95); }
.periph-empty { text-align: center; padding: 24px; font-size: 12px; color: rgba(255,255,255,0.35); }

/* Stats */
#stats { position: fixed; top: 6px; right: 6px; z-index: 30; background: rgba(15,18,37,0.9); backdrop-filter: blur(20px); border: 1px solid rgba(255,255,255,0.06); border-radius: 10px; padding: 10px 14px; font-size: 10px; font-family: monospace; color: rgba(255,255,255,0.35); display: none; min-width: 180px; line-height: 1.7; }

/* Shadow mode badge */
#shadow-badge { position: fixed; top: 12px; left: 50%; transform: translateX(-50%); z-index: 200; background: rgba(239,68,68,0.15); backdrop-filter: blur(12px); border: 1px solid rgba(239,68,68,0.3); border-radius: 8px; padding: 6px 16px; font-size: 12px; font-weight: 600; color: rgba(239,68,68,0.9); display: none; pointer-events: none; letter-spacing: 0.05em; }
body.shadow-mode #shadow-badge { display: block; }
body.shadow-mode #display canvas { cursor: default !important; }
body.shadow-mode .sb-interactive { display: none !important; }
body.shadow-mode .sb-btn.destroy { display: none !important; }
body.shadow-mode #settings-panel { display: none !important; }
body.shadow-mode .sb-section-title { display: none !important; }
body.shadow-mode .sb-section { display: none !important; }
body.shadow-mode .dc-destroy-btn { display: none !important; }
</style>
<script>var module = { exports: {} };</script>
<script src="https://cdn.jsdelivr.net/npm/guacamole-common-js@1.5.0/dist/cjs/guacamole-common.js" integrity="sha384-puRyg8V7m9K0cq9y+ndybD9ZZ08eYGgFwTTnNe8NxMJBUy02vcil7/XWwKa0Rbcs" crossorigin="anonymous"></script>
<script>var Guacamole = module.exports;</script>
</head>
<body>
<textarea id="virtual-kb-input" autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false" style="position:fixed;bottom:0;left:0;width:1px;height:1px;opacity:0.01;z-index:-1;"></textarea>
<div id="confirm-modal">
  <div class="modal-backdrop" onclick="closeConfirmModal()"></div>
  <div class="modal-box">
    <div class="modal-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M15 9l-6 6M9 9l6 6"/></svg></div>
    <div class="modal-title" data-t="destroy">T_DESTROY</div>
    <div class="modal-msg" data-t="destroy_confirm_msg">T_DESTROY_CONFIRM_MSG</div>
    <div class="modal-btns">
      <button class="modal-btn cancel" data-t="cancel" onclick="closeConfirmModal()">T_CANCEL</button>
      <button class="modal-btn danger" data-t="destroy" onclick="closeConfirmModal();destroySession();">T_DESTROY</button>
    </div>
  </div>
</div>
<div id="disconnect-overlay">
  <div id="disconnect-box">
    <div class="dc-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18.36 5.64a9 9 0 11-12.73 0M12 2v10"/></svg></div>
    <h2 data-t="disconnected_title">T_DISCONNECTED_TITLE</h2>
    <p data-t="disconnected_msg">T_DISCONNECTED_MSG</p>
    <div class="dc-btns">
      <button class="dc-btn primary" data-t="reconnect" onclick="reconnectSession()">T_RECONNECT</button>
      <button class="dc-btn secondary" data-t="back" onclick="goBack()">T_BACK</button>
      <button class="dc-btn danger dc-destroy-btn" data-t="destroy" onclick="destroySession()">T_DESTROY</button>
    </div>
  </div>
</div>
<div id="overlay"><div class="spinner"></div><p id="status">Connecting...</p></div>
<div class="sb-reconnecting" id="reconnect-toast" data-t="reconnecting">T_RECONNECTING</div>
<div id="upload-modal">
  <div id="upload-backdrop" onclick="closeUploadModal()"></div>
  <div id="upload-box">
    <button id="upload-close" onclick="closeUploadModal()">&times;</button>
    <h3 data-t="upload_files">T_UPLOAD_FILES</h3>
    <div id="upload-dropzone">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M17 8l-5-5-5 5M12 3v12"/></svg>
      <p><span data-t="upload_drop">T_UPLOAD_DROP</span> <span data-t="upload_browse" onclick="document.getElementById('upload-input').click()">T_UPLOAD_BROWSE</span></p>
    </div>
    <input type="file" id="upload-input" multiple style="display:none">
    <div id="upload-list"></div>
  </div>
</div>
<div id="periph-modal">
  <div id="periph-backdrop" onclick="closePeripheralsModal()"></div>
  <div id="periph-box">
    <button id="periph-close" onclick="closePeripheralsModal()">&times;</button>
    <button id="periph-reload" onclick="refreshPeripheralsList()" title="Reload">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 019-9 9 9 0 016.7 3M21 12a9 9 0 01-9 9 9 9 0 01-6.7-3M21 3v6h-6M3 21v-6h6"/></svg>
    </button>
    <h3 data-t="peripherals">T_PERIPHERALS</h3>
    <div id="periph-list"></div>
    <div id="periph-add-row"></div>
  </div>
</div>
<div id="display"></div>
<div id="shadow-badge" data-t="shadow_mode">T_SHADOW_MODE</div>
<div id="side-tab"><svg viewBox="0 0 24 24"><path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z"/></svg></div>
<div id="sidebar">
  <div class="sb-hdr">
    <svg class="sb-logo-svg" viewBox="0 0 1080 1080"><defs><linearGradient id="lg" x1="0%" x2="100%" y1="0%" y2="100%"><stop offset="0%" stop-color="#7096ff"/><stop offset="100%" stop-color="#65d5c5"/></linearGradient></defs><g transform="matrix(0.18 0 0 0.17 540 540)"><path transform="translate(-5061.17, -5084.07)" d="M 3235 7886 C 3044 7850 2884 7784 2755 7688 C 2546 7530 2405 7326 2343 7090 C 2319 7001 2317 6972 2309 6650 C 2298 6179 2299 3487 2311 3320 C 2321 3163 2344 3062 2399 2939 C 2535 2630 2799 2399 3109 2316 C 3285 2269 3284 2269 5125 2272 L 6865 2275 L 6966 2302 C 7170 2356 7330 2443 7468 2574 C 7642 2739 7749 2929 7799 3163 C 7820 3261 7820 3264 7821 5083 C 7821 6381 7818 6921 7810 6960 C 7752 7250 7664 7418 7479 7596 C 7342 7726 7186 7812 6981 7869 L 6885 7895 L 5095 7897 C 3674 7898 3291 7896 3235 7886 z M 5570 7419 C 5652 7406 5680 7389 5680 7351 C 5680 7335 5674 7318 5667 7312 C 5660 7306 5594 7285 5520 7265 C 5446 7245 5336 7210 5275 7188 C 4950 7069 4821 6961 4594 6617 C 4417 6350 4116 5889 3990 5693 C 3932 5604 3882 5531 3877 5530 C 3866 5530 3822 5635 3794 5730 C 3759 5846 3750 6027 3774 6120 C 3824 6315 3937 6437 4139 6510 L 4213 6537 L 4266 6631 C 4400 6867 4554 7062 4685 7162 C 4878 7308 5048 7388 5239 7420 C 5308 7432 5496 7431 5570 7419 z M 4370 7154 C 4370 7151 4341 7102 4307 7047 C 4272 6991 4235 6931 4225 6913 C 4209 6886 4198 6880 4161 6875 C 4007 6854 3842 6789 3731 6705 C 3607 6610 3492 6431 3454 6271 C 3430 6165 3432 5999 3459 5895 C 3484 5803 3569 5619 3629 5532 C 3694 5436 3771 5359 3959 5201 C 4207 4995 4315 4904 4490 4755 C 4574 4684 4686 4590 4739 4547 C 4901 4414 4940 4367 4940 4304 C 4940 4220 4879 4140 4816 4140 C 4778 4140 4772 4145 4445 4425 C 4262 4581 4052 4759 3705 5051 C 3490 5232 3341 5438 3240 5694 C 3121 5996 3141 6322 3297 6609 C 3365 6736 3413 6793 3528 6885 C 3703 7025 3903 7114 4110 7144 C 4208 7158 4370 7164 4370 7154 z M 6102 7126 C 6329 7082 6544 6966 6700 6803 C 6802 6698 6870 6598 6921 6481 C 6984 6334 7002 6246 7007 6060 L 7012 5896 L 6979 5921 C 6960 5935 6901 5979 6848 6019 C 6754 6090 6751 6093 6745 6138 C 6728 6272 6689 6377 6614 6492 C 6510 6653 6355 6768 6150 6836 C 6075 6861 6050 6864 5930 6864 C 5815 6865 5784 6861 5720 6842 C 5597 6803 5479 6736 5394 6657 C 5294 6564 5228 6477 5016 6160 C 4618 5565 4459 5333 4426 5299 C 4404 5277 4373 5258 4346 5250 C 4305 5239 4297 5240 4257 5259 C 4233 5271 4208 5290 4201 5303 C 4179 5346 4171 5391 4181 5417 C 4190 5440 4415 5787 4430 5800 C 4440 5809 4560 5983 4695 6185 C 4725 6229 4767 6290 4789 6320 C 4811 6350 4867 6431 4914 6500 C 5109 6791 5257 6930 5485 7040 C 5572 7082 5684 7119 5770 7133 C 5831 7143 6034 7139 6102 7126 z M 5945 6535 C 6164 6489 6333 6321 6386 6095 L 6404 6019 L 6553 5937 C 6932 5728 7171 5477 7260 5195 C 7291 5097 7297 4935 7276 4810 C 7255 4686 7234 4616 7214 4599 C 7188 4578 7134 4583 7104 4611 C 7052 4658 7040 4702 7040 4851 C 7040 5025 7029 5084 6974 5197 C 6939 5270 6913 5307 6845 5378 C 6760 5468 6728 5491 6477 5646 C 6354 5722 6105 5913 5687 6255 C 5464 6437 5470 6421 5591 6485 C 5708 6546 5818 6562 5945 6535 z M 5421 5982 C 5642 5788 6029 5459 6310 5223 C 6456 5102 6613 4963 6658 4916 C 6799 4769 6911 4573 6961 4383 C 6988 4284 6998 4061 6980 3954 C 6948 3753 6830 3530 6671 3370 C 6456 3153 6157 3043 5809 3052 L 5683 3055 L 5761 3185 L 5840 3315 L 5933 3322 C 6182 3341 6407 3454 6550 3633 C 6616 3716 6686 3858 6706 3950 C 6727 4048 6728 4216 6706 4298 C 6665 4459 6588 4589 6458 4722 C 6393 4787 6135 5009 5687 5384 C 5444 5587 5203 5799 5187 5824 C 5158 5868 5147 5932 5160 5981 C 5173 6030 5236 6100 5267 6100 C 5278 6100 5347 6047 5421 5982 z M 3045 5576 C 3064 5568 3091 5544 3107 5519 C 3134 5478 3135 5474 3141 5286 C 3146 5114 3150 5087 3174 5015 C 3241 4816 3360 4685 3624 4520 C 3841 4384 3915 4331 4146 4153 C 4251 4072 4396 3960 4468 3905 C 4540 3850 4598 3801 4596 3795 C 4595 3790 4565 3766 4531 3743 C 4443 3683 4382 3667 4250 3672 C 4148 3675 4139 3677 4059 3717 C 3905 3793 3813 3909 3770 4077 C 3761 4114 3754 4122 3714 4140 C 3637 4173 3440 4294 3351 4361 C 3064 4578 2900 4847 2883 5130 C 2879 5201 2883 5255 2898 5346 C 2924 5494 2947 5560 2980 5577 C 3011 5593 3004 5593 3045 5576 z M 5130 5292 C 5182 5272 5234 5210 5251 5147 C 5266 5092 5249 5009 5214 4962 C 5133 4856 4944 4865 4865 4978 C 4839 5017 4835 5030 4836 5090 C 4836 5183 4872 5243 4949 5283 C 5012 5314 5064 5317 5130 5292 z M 5811 4950 C 5854 4938 5898 4894 5905 4855 C 5913 4815 5908 4808 5671 4467 C 5440 4137 5343 3997 5180 3760 C 5117 3669 5039 3559 5006 3515 C 4845 3303 4638 3163 4397 3103 C 4305 3080 4281 3078 4150 3083 C 3988 3088 3901 3108 3755 3175 C 3544 3272 3374 3440 3262 3665 C 3193 3805 3163 3916 3150 4083 C 3137 4256 3132 4254 3285 4143 C 3407 4054 3415 4045 3422 4007 C 3476 3709 3711 3447 3992 3373 C 4069 3353 4243 3350 4320 3369 C 4350 3376 4422 3404 4480 3432 C 4576 3478 4594 3491 4690 3589 C 4766 3665 4829 3744 4916 3870 C 4982 3966 5051 4065 5069 4090 C 5087 4115 5130 4178 5166 4230 C 5246 4348 5285 4404 5389 4550 C 5434 4613 5486 4689 5505 4717 C 5567 4812 5659 4917 5692 4933 C 5720 4946 5738 4952 5770 4959 C 5773 4959 5791 4955 5811 4950 z M 6283 4565 C 6345 4440 6364 4361 6364 4215 C 6364 4119 6360 4081 6342 4024 C 6282 3828 6103 3677 5915 3663 C 5853 3658 5836 3653 5824 3636 C 5815 3624 5751 3527 5681 3420 C 5610 3313 5522 3189 5484 3145 C 5274 2904 4929 2753 4636 2774 C 4470 2787 4410 2811 4410 2865 C 4410 2898 4435 2909 4600 2954 C 4899 3035 5118 3153 5264 3311 C 5314 3364 5371 3445 5655 3855 C 5697 3916 5797 4060 5877 4175 C 5958 4291 6057 4435 6098 4495 C 6204 4651 6221 4673 6230 4663 C 6235 4659 6259 4615 6283 4565 z" fill="url(#lg)"/></g></svg>
    <span class="sb-logo-text">Oklavier</span>
    <button class="sb-x" style="margin-left:auto" onclick="document.body.classList.remove('sb-open')">&times;</button>
  </div>
  <div class="sb-scroll">
  <div class="sb-menu">
    <div class="sb-btn" onclick="toggleFS()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M8 3H5a2 2 0 00-2 2v3m18 0V5a2 2 0 00-2-2h-3m0 18h3a2 2 0 002-2v-3M3 16v3a2 2 0 002 2h3"/></svg>
      <span data-t="fullscreen">T_FULLSCREEN</span>
    </div>
    <div class="sb-btn sb-interactive" onclick="openUploadModal()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M17 8l-5-5-5 5M12 3v12"/></svg>
      <span data-t="upload_files">T_UPLOAD_FILES</span>
    </div>
    <div class="sb-btn sb-interactive" onclick="openPeripheralsModal()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 19v-3a4 4 0 014-4h2M9 5v3M15 5v3M5 9h14M5 9a2 2 0 00-2 2v8a2 2 0 002 2h6M5 9V7a2 2 0 012-2h10a2 2 0 012 2v2M19 13v8M16 17h6"/></svg>
      <span data-t="peripherals_redirect">T_PERIPHERALS_REDIRECT</span>
    </div>
    <div class="sb-btn sb-interactive" onclick="sendCAD()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M7 8h10M7 12h6"/></svg>
      <span data-t="ctrl_alt_del">T_CTRL_ALT_DEL</span>
    </div>
    <div class="sb-btn sb-interactive" id="btn-vkb" style="display:none" onclick="toggleVirtualKeyboard()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="6" width="20" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M8 14h8"/></svg>
      <span data-t="virtual_keyboard">T_VIRTUAL_KEYBOARD</span>
    </div>
    <script>if('ontouchstart' in window||navigator.maxTouchPoints>0)document.getElementById('btn-vkb').style.display='';</script>
  </div>
  <div class="sb-div"></div>
  <div class="sb-section-title" data-t="settings">T_SETTINGS</div>
  <div class="sb-section" id="settings-panel">
    <div class="sb-toggle" onclick="toggleSetting('enable-font-smoothing')"><span class="sb-toggle-label" data-t="font_smoothing">T_FONT_SMOOTHING</span><div class="sb-toggle-switch on" id="sw-enable-font-smoothing"></div></div>
    <div class="sb-toggle" onclick="toggleSetting('enable-wallpaper')"><span class="sb-toggle-label" data-t="wallpaper">T_WALLPAPER</span><div class="sb-toggle-switch on" id="sw-enable-wallpaper"></div></div>
    <div class="sb-toggle" onclick="toggleSetting('enable-theming')"><span class="sb-toggle-label" data-t="theming">T_THEMING</span><div class="sb-toggle-switch on" id="sw-enable-theming"></div></div>
    <div class="sb-toggle" onclick="toggleSetting('enable-desktop-composition')"><span class="sb-toggle-label" data-t="desktop_effects">T_DESKTOP_EFFECTS</span><div class="sb-toggle-switch on" id="sw-enable-desktop-composition"></div></div>
    <div class="sb-toggle" onclick="toggleSetting('enable-full-window-drag')"><span class="sb-toggle-label" data-t="window_drag">T_WINDOW_DRAG</span><div class="sb-toggle-switch on" id="sw-enable-full-window-drag"></div></div>
    <div class="sb-toggle" onclick="toggleSetting('enable-menu-animations')"><span class="sb-toggle-label" data-t="menu_animations">T_MENU_ANIMATIONS</span><div class="sb-toggle-switch on" id="sw-enable-menu-animations"></div></div>
    <div class="sb-toggle" onclick="toggleAudio()"><span class="sb-toggle-label" data-t="audio">T_AUDIO</span><div class="sb-toggle-switch on" id="sw-enable-audio"></div></div>
    <div class="sb-toggle" onclick="toggleSetting('enable-clipboard')"><span class="sb-toggle-label" data-t="clipboard">T_CLIPBOARD</span><div class="sb-toggle-switch on" id="sw-enable-clipboard"></div></div>
    <div class="sb-toggle" onclick="toggleSetting('remote-cursor')"><span class="sb-toggle-label" data-t="remote_cursor">T_REMOTE_CURSOR</span><div class="sb-toggle-switch on" id="sw-remote-cursor"></div></div>
  </div>
  <div class="sb-div"></div>
  <div class="sb-section">
    <div class="sb-select">
      <label data-t="keyboard_layout">T_KEYBOARD_LAYOUT</label>
      <select id="sel-keyboard" onchange="changeSetting('server-layout', this.value)">
        <option value="en-us-qwerty">English (US) - QWERTY</option>
        <option value="en-gb-qwerty">English (UK) - QWERTY</option>
        <option value="fr-fr-azerty">Fran&#231;ais - AZERTY</option>
        <option value="fr-be-azerty">Fran&#231;ais (BE) - AZERTY</option>
        <option value="de-de-qwertz">Deutsch - QWERTZ</option>
        <option value="es-es-qwerty">Espa&#241;ol - QWERTY</option>
        <option value="it-it-qwerty">Italiano - QWERTY</option>
        <option value="pt-br-qwerty">Portugu&#234;s (BR) - QWERTY</option>
        <option value="sv-se-qwerty">Svenska - QWERTY</option>
        <option value="ja-jp-qwerty">Japanese - QWERTY</option>
      </select>
    </div>
    <div class="sb-select">
      <label data-t="language">T_LANGUAGE</label>
      <select id="sel-language" onchange="changeLanguage(this.value)">
        <option value="en">English</option>
        <option value="fr">Fran&#231;ais</option>
        <option value="de">Deutsch</option>
        <option value="es">Espa&#241;ol</option>
      </select>
    </div>
    <div class="sb-select">
      <label data-t="display_scale">T_DISPLAY_SCALE</label>
      <div style="display:flex;align-items:center;gap:8px;">
        <input type="range" id="sel-scale" min="50" max="200" step="10" value="100" style="flex:1;accent-color:#7096ff;" onchange="changeScale(this.value)" oninput="document.getElementById('scale-val').textContent=this.value+'%'">
        <span id="scale-val" style="font-size:11px;color:rgba(255,255,255,0.5);min-width:35px;">100%</span>
      </div>
    </div>
    <div class="sb-select">
      <label data-t="color_depth">T_COLOR_DEPTH</label>
      <select id="sel-color-depth" onchange="changeSetting('color-depth', this.value)">
        <option value="16" data-t-template="fast">16-bit (T_FAST)</option>
        <option value="24">24-bit</option>
        <option value="32" selected data-t-template="quality">32-bit (T_QUALITY)</option>
      </select>
    </div>
  </div>
  </div>
  <div class="sb-ft">
    <div class="sb-btn" onclick="goBack()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 12H5M12 19l-7-7 7-7"/></svg>
      <span data-t="back">T_BACK</span>
    </div>
    <div class="sb-btn destroy" onclick="openConfirmModal()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M15 9l-6 6M9 9l6 6"/></svg>
      <span data-t="destroy">T_DESTROY</span>
    </div>
  </div>
  <div class="sb-info">Session SESSION_ID_PLACEHOLDER<br>Oklavier v1.0.2</div>
</div>

<script>
var SID = 'SESSION_ID_PLACEHOLDER';
var CP = 'CONTROL_PLANE_PLACEHOLDER';
var PROTO = 'PROTOCOL_PLACEHOLDER';
var TRANSLATIONS = ALL_TRANSLATIONS_PLACEHOLDER;
var LANG = localStorage.getItem('oklavier-viewer-lang') || 'LANG_PLACEHOLDER';
var T = TRANSLATIONS[LANG] || TRANSLATIONS.en;
var DFCFG = __DFCFG__;
var SHADOW = __SHADOW__;
var SESSION_TOKEN = '';

// Auto-reconnect state
var autoReconnectAttempts = 0;
var maxAutoReconnect = 5;
var autoReconnectTimer = null;

function changeLanguage(lang) {
  T = TRANSLATIONS[lang] || TRANSLATIONS.en;
  LANG = lang;
  localStorage.setItem('oklavier-viewer-lang', lang);
  updateUITexts();
}

function updateUITexts() {
  document.querySelectorAll('[data-t]').forEach(function(el) {
    var key = el.getAttribute('data-t');
    if (T[key] !== undefined) el.textContent = T[key];
  });
  // Update color depth options with template values
  document.querySelectorAll('[data-t-template]').forEach(function(el) {
    var key = el.getAttribute('data-t-template');
    var bits = el.value;
    if (T[key] !== undefined) el.textContent = bits + '-bit (' + T[key] + ')';
  });
  // Update document title
  if (SHADOW) document.title = 'Oklavier - ' + T.shadow_mode;
}

// Initialize language selector
(function() {
  var selLang = document.getElementById('sel-language');
  if (selLang) selLang.value = LANG;
  // Apply saved language on load (overrides server-provided language)
  if (localStorage.getItem('oklavier-viewer-lang')) updateUITexts();
})();

// Apply shadow mode class to body + hide interactive controls
if (SHADOW) {
  document.body.classList.add('shadow-mode');
  document.title = 'Oklavier - ' + T.shadow_mode;
  // Hide interactive sidebar buttons (upload, ctrl+alt+del, virtual keyboard, destroy)
  document.querySelectorAll('.sb-btn.destroy').forEach(function(el) { el.style.display = 'none'; });
  // Hide confirm modal, upload modal elements
  var confirmModal = document.getElementById('confirm-modal');
  if (confirmModal) confirmModal.style.display = 'none';
  var uploadModal = document.getElementById('upload-modal');
  if (uploadModal) uploadModal.style.display = 'none';
}

// Extract token from URL hash
(function() {
  var hash = location.hash;
  if (hash && hash.indexOf('token=') > -1) {
    SESSION_TOKEN = hash.split('token=')[1];
    history.replaceState(null, '', location.pathname); // clean URL
  }
})();

// UI
function toggleFS() { document.fullscreenElement ? document.exitFullscreen() : document.documentElement.requestFullscreen(); }

// Close sidebar when clicking on the display
document.getElementById('display').addEventListener('mousedown', function() {
  document.body.classList.remove('sb-open');
});

// Draggable side tab (vertical)
(function() {
  var tab = document.getElementById('side-tab');
  var startY, startTop, dragging = false;
  var savedTop = localStorage.getItem('oklavier-tab-top');
  if (savedTop) { tab.style.top = savedTop + 'px'; tab.style.transform = 'none'; }
  else { tab.style.top = '50%'; tab.style.transform = 'translateY(-50%)'; }
  tab.addEventListener('mousedown', function(e) { startY = e.clientY; startTop = tab.offsetTop; dragging = false; });
  document.addEventListener('mousemove', function(e) {
    if (startY === undefined) return;
    if (Math.abs(e.clientY - startY) > 5) dragging = true;
    if (dragging) {
      var newTop = Math.max(20, Math.min(window.innerHeight - 76, startTop + e.clientY - startY));
      tab.style.top = newTop + 'px';
      tab.style.transform = 'none';
    }
  });
  document.addEventListener('mouseup', function() {
    if (dragging) { localStorage.setItem('oklavier-tab-top', tab.offsetTop); }
    else if (startY !== undefined) { document.body.classList.toggle('sb-open'); }
    startY = undefined; dragging = false;
  });
})();
function goBack() {
  if (SHADOW) { window.close(); return; }
  window.location.href = (CP || '').replace(/\/api.*/, '') + '/workspaces';
}
function reconnectSession() {
  isReconnecting = true;
  autoReconnectAttempts = 0;
  if (autoReconnectTimer) { clearTimeout(autoReconnectTimer); autoReconnectTimer = null; }
  document.getElementById('disconnect-overlay').classList.remove('show');
  document.getElementById('reconnect-toast').classList.remove('show');
  document.getElementById('overlay').classList.remove('hidden');
  setStatus(T.reconnecting);
  var display = document.getElementById('display');
  while (display.firstChild) display.removeChild(display.firstChild);
  setTimeout(connect, 500);
}
function destroySession() {
  if (SHADOW) { goBack(); return; }
  fetch('/sessions/' + SID + '/destroy', { method: 'POST', headers: { 'Authorization': 'Bearer ' + SESSION_TOKEN } })
    .then(function() { goBack(); })
    .catch(function() { goBack(); });
}
var statsOn = false;
function toggleStats() { statsOn = !statsOn; document.getElementById('stats').style.display = statsOn ? 'block' : 'none'; }

function setStatus(text) { document.getElementById('status').textContent = text; }

// Guacamole client
var tunnel = null;
var guac = null;
var connected = false;

function connect() {
  setStatus(T.connecting + ' ' + PROTO.toUpperCase() + '...');

  var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  var wsUrl = proto + '//' + location.host + '/guac-ws/' + SID;

  // SECURITY: SESSION_TOKEN is now a single-use random bearer issued by the
  // core (TTL 60s, push-admitted to this agent server-to-server). It is NOT
  // a long-lived JWT. We pass it as ?ticket= rather than ?token= so it goes
  // through the ticket consumption path (single-use; can't be replayed).
  tunnel = new Guacamole.Tunnel();
  var wsParams = 'w=' + window.innerWidth + '&h=' + window.innerHeight;
  if (SESSION_TOKEN) wsParams += '&ticket=' + encodeURIComponent(SESSION_TOKEN);
  var wsAuthUrl = wsUrl + '?' + wsParams;
  var socket = new WebSocket(wsAuthUrl);
  var parser = new Guacamole.Parser();

  parser.oninstruction = function(opcode, args) {
    if (tunnel.oninstruction) tunnel.oninstruction(opcode, args);
  };

  tunnel.sendMessage = function() {
    if (socket.readyState !== 1) return;
    var msg = '';
    for (var i = 0; i < arguments.length; i++) {
      var val = String(arguments[i]);
      msg += val.length + '.' + val;
      if (i < arguments.length - 1) msg += ',';
    }
    msg += ';';
    socket.send(msg);
  };

  tunnel.connect = function() {};
  tunnel.disconnect = function() { socket.close(); };

  socket.onopen = function() {
    tunnel.state = Guacamole.Tunnel.State.OPEN;
    if (tunnel.onstatechange) tunnel.onstatechange(Guacamole.Tunnel.State.OPEN);
  };
  socket.onmessage = function(e) { parser.receive(e.data); };
  socket.onclose = function(e) {
    tunnel.state = Guacamole.Tunnel.State.CLOSED;
    if (tunnel.onstatechange) tunnel.onstatechange(Guacamole.Tunnel.State.CLOSED);
  };
  socket.onerror = function() {
    if (tunnel.onerror) tunnel.onerror(new Guacamole.Status(Guacamole.Status.Code.SERVER_ERROR));
  };

  guac = new Guacamole.Client(tunnel);

  var display = document.getElementById('display');
  display.appendChild(guac.getDisplay().getElement());

  guac.onstatechange = function(state) {
    switch(state) {
      case 1: setStatus('Connecting...'); break;
      case 2: setStatus('Waiting for server...'); break;
      case 3:
        connected = true;
        isReconnecting = false;
        autoReconnectAttempts = 0;
        if (autoReconnectTimer) { clearTimeout(autoReconnectTimer); autoReconnectTimer = null; }
        document.getElementById('overlay').classList.add('hidden');
        document.getElementById('reconnect-toast').classList.remove('show');
        // Send viewport size to trigger server resize, then scale display to fit
        sendSize();
        setTimeout(resize, 200);
        setTimeout(function() { sendSize(); resize(); }, 1000);
        // Self-heal in case the VNC server wasn't ready to RandR yet.
        healAttempts = 0;
        startHealLoop();
        break;
      case 4:
      case 5:
        connected = false;
        if (!isReconnecting) {
          if (autoReconnectAttempts < maxAutoReconnect) {
            autoReconnectAttempts++;
            var delay = Math.min(Math.pow(2, autoReconnectAttempts - 1) * 1000, 30000);
            var toast = document.getElementById('reconnect-toast');
            toast.textContent = T.reconnecting + ' (' + autoReconnectAttempts + '/' + maxAutoReconnect + ')';
            toast.classList.add('show');
            document.getElementById('overlay').classList.add('hidden');
            autoReconnectTimer = setTimeout(function() {
              var display = document.getElementById('display');
              while (display.firstChild) display.removeChild(display.firstChild);
              connect();
            }, delay);
          } else {
            document.getElementById('reconnect-toast').classList.remove('show');
            document.getElementById('overlay').classList.add('hidden');
            document.getElementById('disconnect-overlay').classList.add('show');
          }
        }
        break;
    }
  };

  guac.onerror = function(status) {
    setStatus('Error: ' + (status.message || status.code || status));
  };

  // Keyboard (disabled in shadow mode)
  if (!SHADOW) {
    var keyboard = new Guacamole.Keyboard(document);
    keyboard.onkeydown = function(keysym) { guac.sendKeyEvent(1, keysym); };
    keyboard.onkeyup = function(keysym) { guac.sendKeyEvent(0, keysym); };
  }

  // Mouse — hide browser cursor, show remote cursor from server
  var displayEl = guac.getDisplay().getElement();
  if (!SHADOW) {
    var mouse = new Guacamole.Mouse(displayEl);
    mouse.onmousedown = mouse.onmouseup = mouse.onmousemove = function(mouseState) {
      // Scale mouse position to match display scale
      var scale = guac.getDisplay().getScale();
      var scaledState = new Guacamole.Mouse.State(
        mouseState.x / scale, mouseState.y / scale,
        mouseState.left, mouseState.middle, mouseState.right, mouseState.up, mouseState.down
      );
      guac.sendMouseState(scaledState);
    };
  }
  // Use CSS cursor from remote cursor image (native browser cursor, zero latency)
  guac.getDisplay().oncursor = function(canvas, x, y) {
    if (settings['remote-cursor'] === 'true') {
      displayEl.style.cursor = 'url(' + canvas.toDataURL('image/png') + ') ' + x + ' ' + y + ', default';
    } else {
      displayEl.style.cursor = 'default';
    }
  };

  // Touch (for mobile, disabled in shadow mode)
  if (!SHADOW) {
    var touch = new Guacamole.Mouse.Touchscreen(guac.getDisplay().getElement());
    touch.onmousedown = touch.onmouseup = touch.onmousemove = function(mouseState) {
      guac.sendMouseState(mouseState);
    };
  }

  guac.onaudio = function(stream, mimetype) {
    if (audioMuted) {
      stream.sendEnd();
      return null;
    }
    if (!Guacamole.AudioPlayer || !Guacamole.AudioPlayer.getSupportedTypes().length) {
      stream.sendEnd();
      return null;
    }
    var player = Guacamole.AudioPlayer.getInstance(stream, mimetype);
    if (player) {
      audioPlayers.push(player);
      stream.onend = function() {
        var idx = audioPlayers.indexOf(player);
        if (idx >= 0) audioPlayers.splice(idx, 1);
      };
    }
    return player;
  };

  // Auto-resize: scale display to fit viewport
  var resizeTimeout = null;
  var lastSentW = 0, lastSentH = 0;

  function resize() {
    var w = window.innerWidth;
    var h = window.innerHeight;
    var dw = guac.getDisplay().getWidth();
    var dh = guac.getDisplay().getHeight();
    if (!dw || !dh) return;
    var s = parseInt(settings['display-scale'] || '100') / 100;
    guac.getDisplay().scale(Math.min(w / dw, h / dh) * s);
  }

  // Only push when the viewport actually changed since last push, otherwise
  // we spam the server with no-ops every 500ms during the heal loop.
  function sendSize() {
    var w = window.innerWidth, h = window.innerHeight;
    if (w < 100 || h < 100) return;
    if (w === lastSentW && h === lastSentH) return;
    lastSentW = w; lastSentH = h;
    guac.sendSize(w, h);
  }

  // Self-heal: VNC servers often aren't ready to RandR right when the Guac
  // socket flips to state=3, so the initial sendSize() is dropped silently.
  // Re-check the display vs viewport for a few seconds after connect; if they
  // disagree (or the display reports 0×0 because no first frame yet), push
  // again. Stops once display matches viewport (within 8px slack for the
  // resize-method=display-update rounding).
  var healAttempts = 0;
  function startHealLoop() {
    if (window._gucHeal) return;
    window._gucHeal = setInterval(function() {
      if (!connected) { stopHealLoop(); return; }
      var w = window.innerWidth, h = window.innerHeight;
      var dw = guac.getDisplay().getWidth(), dh = guac.getDisplay().getHeight();
      var mismatch = !dw || !dh || Math.abs(dw - w) > 8 || Math.abs(dh - h) > 8;
      if (mismatch) {
        // sendSize() de-dupes by lastSentW/H — clear so we force a push.
        lastSentW = 0; lastSentH = 0;
        sendSize();
        resize();
      }
      if (++healAttempts > 20 || !mismatch) stopHealLoop(); // 10s cap
    }, 500);
  }
  function stopHealLoop() {
    if (window._gucHeal) { clearInterval(window._gucHeal); window._gucHeal = null; }
  }

  window.addEventListener('resize', function() {
    resize();
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(function() { sendSize(); resize(); }, 300);
  });

  // Some browsers don't fire 'resize' on fullscreen entry/exit reliably.
  document.addEventListener('fullscreenchange', function() {
    setTimeout(function() { sendSize(); resize(); }, 50);
  });

  // When the popup/tab regains focus, geometry may have shifted (devtools,
  // OS scaling). Cheap re-sync.
  document.addEventListener('visibilitychange', function() {
    if (document.visibilityState === 'visible') {
      setTimeout(function() { sendSize(); resize(); }, 50);
    }
  });

  guac.getDisplay().onresize = function(w, h) {
    resize();
  };

  // Clipboard sync (respects enable-clipboard toggle)
  // Remote → Local
  guac.onclipboard = function(stream, mimetype) {
    if (settings['enable-clipboard'] !== 'true') return;
    if (mimetype === 'text/plain') {
      var reader = new Guacamole.StringReader(stream);
      var data = '';
      reader.ontext = function(text) { data += text; };
      reader.onend = function() {
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(data);
        }
      };
    }
  };

  // Local → Remote: send clipboard to guac
  function sendClipboardToGuac(text) {
    if (!guac || !connected || settings['enable-clipboard'] !== 'true') return;
    var stream = guac.createClipboardStream('text/plain');
    var writer = new Guacamole.StringWriter(stream);
    writer.sendText(text);
    writer.sendEnd();
  }

  // Intercept paste events (Ctrl+V)
  window.addEventListener('paste', function(e) {
    if (settings['enable-clipboard'] !== 'true') return;
    var text = (e.clipboardData || window.clipboardData).getData('text');
    if (text) sendClipboardToGuac(text);
  });

  // Sync clipboard on window focus (requires clipboard permission)
  window.addEventListener('focus', function() {
    if (settings['enable-clipboard'] !== 'true') return;
    if (navigator.clipboard && navigator.clipboard.readText) {
      navigator.clipboard.readText().then(function(text) {
        if (text) sendClipboardToGuac(text);
      }).catch(function() {});
    }
  });

  // File download (printing → PDF, or drive files)
  guac.onfile = function(stream, mimetype, filename) {
    var reader = new Guacamole.BlobReader(stream, mimetype);
    reader.onend = function() {
      var blob = reader.getBlob();
      var url = URL.createObjectURL(blob);
      var a = document.createElement('a');
      a.href = url;
      a.download = filename || 'download';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      setTimeout(function() { URL.revokeObjectURL(url); }, 5000);
    };
    stream.sendAck('OK', 0x0000);
  };

  // Drive redirection: file upload via drag & drop
  var driveObject = null;
  guac.onfilesystem = function(object) {
    driveObject = object;
  };

  // Upload modal logic
  var dropzone = document.getElementById('upload-dropzone');
  var uploadInput = document.getElementById('upload-input');
  var uploadList = document.getElementById('upload-list');

  dropzone.addEventListener('dragover', function(e) { e.preventDefault(); dropzone.classList.add('dragover'); });
  dropzone.addEventListener('dragleave', function() { dropzone.classList.remove('dragover'); });
  dropzone.addEventListener('drop', function(e) {
    e.preventDefault();
    dropzone.classList.remove('dragover');
    if (e.dataTransfer.files.length) uploadFiles(e.dataTransfer.files);
  });
  uploadInput.addEventListener('change', function() {
    if (uploadInput.files.length) uploadFiles(uploadInput.files);
    uploadInput.value = '';
  });

  // Also support drag & drop on the viewer itself
  var displayDiv = document.getElementById('display');
  displayDiv.addEventListener('dragover', function(e) { e.preventDefault(); e.dataTransfer.dropEffect = 'copy'; displayDiv.style.outline = '2px solid rgba(112,150,255,0.5)'; });
  displayDiv.addEventListener('dragleave', function() { displayDiv.style.outline = ''; });
  displayDiv.addEventListener('drop', function(e) {
    e.preventDefault(); displayDiv.style.outline = '';
    if (e.dataTransfer.files.length) { openUploadModal(); uploadFiles(e.dataTransfer.files); }
  });

  function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / 1048576).toFixed(1) + ' MB';
  }

  function uploadFiles(files) {
    if (!driveObject) return;
    for (var i = 0; i < files.length; i++) {
      (function(file) {
        // Create UI item
        var item = document.createElement('div');
        item.className = 'upload-item';
        item.innerHTML = '<div class="upload-item-top"><span class="upload-item-name">' + file.name + '</span><span class="upload-item-size">' + formatSize(file.size) + '</span></div><div class="upload-item-bar"><div class="upload-item-fill"></div></div>';
        uploadList.insertBefore(item, uploadList.firstChild);
        var fill = item.querySelector('.upload-item-fill');

        // Upload via guac drive
        var stream = driveObject.createOutputStream(file.type || 'application/octet-stream', '/' + file.name);
        var writer = new Guacamole.ArrayBufferWriter(stream);
        var CHUNK = 65536;
        var offset = 0;

        function sendNextChunk() {
          var slice = file.slice(offset, offset + CHUNK);
          var reader = new FileReader();
          reader.onload = function() {
            writer.sendData(reader.result);
            offset += reader.result.byteLength;
            var pct = Math.min(100, Math.round(offset / file.size * 100));
            fill.style.width = pct + '%';
            if (offset < file.size) {
              sendNextChunk();
            } else {
              writer.sendEnd();
              item.classList.add('done');
            }
          };
          reader.onerror = function() {
            item.classList.add('error');
          };
          reader.readAsArrayBuffer(slice);
        }
        sendNextChunk();
      })(files[i]);
    }
  }

  guac.connect('');
}

// === Settings (saved in localStorage, reconnect on change) ===
var PREFS_KEY = 'oklavier-rdp-settings';
var TOGGLE_NAMES = ['enable-font-smoothing','enable-wallpaper','enable-theming','enable-desktop-composition','enable-full-window-drag','enable-menu-animations','enable-audio','enable-clipboard','remote-cursor'];
var SELECT_NAMES = ['server-layout','color-depth'];
var SETTING_NAMES = TOGGLE_NAMES.concat(SELECT_NAMES);
var settings = {};

// Audio playback state
var audioPlayers = [];
var audioMuted = false;

// Resume AudioContext (required by browser autoplay policy)
function resumeAudioContext() {
  var AC = window.AudioContext || window.webkitAudioContext;
  if (AC) {
    var ctx = Guacamole.AudioContextFactory ? Guacamole.AudioContextFactory.getAudioContext() : new AC();
    if (ctx && ctx.state === 'suspended') ctx.resume();
  }
}

// Unlock audio on first user interaction (browser autoplay policy)
(function() {
  var unlocked = false;
  function unlock() {
    if (unlocked) return;
    unlocked = true;
    resumeAudioContext();
    document.removeEventListener('click', unlock, true);
    document.removeEventListener('keydown', unlock, true);
    document.removeEventListener('touchstart', unlock, true);
  }
  document.addEventListener('click', unlock, true);
  document.addEventListener('keydown', unlock, true);
  document.addEventListener('touchstart', unlock, true);
})();

function toggleAudio() {
  audioMuted = !audioMuted;
  settings['enable-audio'] = audioMuted ? '' : 'true';
  var sw = document.getElementById('sw-enable-audio');
  if (sw) {
    if (audioMuted) sw.classList.remove('on');
    else sw.classList.add('on');
  }
  if (audioMuted) {
    // End all current audio streams so playback stops immediately
    audioPlayers.slice().forEach(function(p) {
      try { if (p.sync) p.sync(); } catch(e) {}
    });
    audioPlayers = [];
  } else {
    // Resume AudioContext when unmuting
    resumeAudioContext();
  }
  saveSettings();
  // Audio toggle does NOT require reconnect — the onaudio handler
  // checks audioMuted on each new stream from the server
}

// Default keyboard layout based on lang
var LANG_LAYOUTS = {'fr':'fr-fr-azerty','de':'de-de-qwertz','es':'es-es-qwerty','it':'it-it-qwerty','pt':'pt-br-qwerty','sv':'sv-se-qwerty','ja':'ja-jp-qwerty'};
var DEFAULT_LAYOUT = LANG_LAYOUTS['LANG_PLACEHOLDER'] || 'en-us-qwerty';

function loadSettings() {
  try { settings = JSON.parse(localStorage.getItem(PREFS_KEY)) || {}; } catch(e) { settings = {}; }
  var hasUserSettings = Object.keys(settings).length > 0;
  // Defaults for toggles (workspace defaults > global defaults)
  TOGGLE_NAMES.forEach(function(name) {
    if (settings[name] === undefined) settings[name] = (DFCFG[name] !== undefined) ? DFCFG[name] : 'true';
    var sw = document.getElementById('sw-' + name);
    if (sw) {
      if (settings[name] === 'true') sw.classList.add('on');
      else sw.classList.remove('on');
    }
  });
  // Defaults for selects
  if (!settings['server-layout']) settings['server-layout'] = DFCFG['server-layout'] || DEFAULT_LAYOUT;
  if (!settings['color-depth']) settings['color-depth'] = DFCFG['color-depth'] || '32';
  if (!settings['display-scale']) settings['display-scale'] = DFCFG['display-scale'] || '100';
  var selKb = document.getElementById('sel-keyboard');
  if (selKb) selKb.value = settings['server-layout'];
  var selCd = document.getElementById('sel-color-depth');
  if (selCd) selCd.value = settings['color-depth'];
  // Scale slider
  if (!settings['display-scale']) settings['display-scale'] = '100';
  var selScale = document.getElementById('sel-scale');
  if (selScale) { selScale.value = settings['display-scale']; document.getElementById('scale-val').textContent = settings['display-scale'] + '%'; }
  // Auto-detect timezone
  try { settings['timezone'] = Intl.DateTimeFormat().resolvedOptions().timeZone; } catch(e) {}
  // Sync audioMuted with loaded setting
  audioMuted = (settings['enable-audio'] !== 'true');
}

function saveSettings() {
  localStorage.setItem(PREFS_KEY, JSON.stringify(settings));
}

var reconnectTimer = null;
function toggleSetting(name) {
  settings[name] = settings[name] === 'true' ? '' : 'true';
  var sw = document.getElementById('sw-' + name);
  if (sw) sw.classList.toggle('on');
  // Client-side only toggles, no reconnect needed
  if (name === 'enable-clipboard' || name === 'remote-cursor') {
    if (name === 'remote-cursor') applyRemoteCursor();
    saveSettings();
    return;
  }
  saveSettings();
  clearTimeout(reconnectTimer);
  reconnectTimer = setTimeout(reconnectWithSettings, 800);
}

function changeScale(val) {
  settings['display-scale'] = val;
  saveSettings();
  applyScale();
}
function applyScale() {
  if (!guac) return;
  var s = parseInt(settings['display-scale'] || '100') / 100;
  var dw = guac.getDisplay().getWidth();
  var dh = guac.getDisplay().getHeight();
  if (dw && dh) {
    var fitScale = Math.min(window.innerWidth / dw, window.innerHeight / dh);
    guac.getDisplay().scale(fitScale * s);
  }
}
function applyRemoteCursor() {
  var el = document.querySelector('#display > div');
  if (el) el.style.cursor = settings['remote-cursor'] === 'true' ? 'none' : 'default';
}

function openConfirmModal() { document.getElementById('confirm-modal').classList.add('show'); document.body.classList.remove('sb-open'); }
function closeConfirmModal() { document.getElementById('confirm-modal').classList.remove('show'); }
function openUploadModal() { document.getElementById('upload-modal').classList.add('show'); document.body.classList.remove('sb-open'); }
function closeUploadModal() { document.getElementById('upload-modal').classList.remove('show'); }

// Peripherals modal — Phase 1.0 scaffolding.
// Lists detected media devices + already-paired WebUSB/WebHID/Gamepad with
// real names, lets the user toggle redirection. The toggle currently only
// emits a {ch:'peripherals',type:'enable|disable',id,kind} message on the
// existing guacd WS so we can wire the agent backend later — actual
// stream forwarding (webcam/mic/USB → VM) is Phase 1.1.
var peripheralsEnabled = {}; // id -> {kind, label, stream/device handle}
var peripheralsPermsAsked = false;
// Some APIs don't expose an "already-granted" enumeration (File System
// Access, Display Capture, MIDI in some browsers). We track what the user
// added during this session so they show up in the modal alongside the
// auto-enumerated devices.
var peripheralsExtras = []; // [{id, kind, label, handle}]
var peripheralsMidiAccess = null;
var peripheralsGeoEnabled = false;
var peripheralsNfcEnabled = false;

async function openPeripheralsModal() {
  document.getElementById('periph-modal').classList.add('show');
  document.body.classList.remove('sb-open');
  await refreshPeripheralsList();
}
function closePeripheralsModal() {
  document.getElementById('periph-modal').classList.remove('show');
}

async function refreshPeripheralsList() {
  var list = document.getElementById('periph-list');
  list.innerHTML = '';

  var hasMedia = !!(navigator.mediaDevices && navigator.mediaDevices.enumerateDevices);
  var mediaDevs = [];
  if (hasMedia) {
    try { mediaDevs = await navigator.mediaDevices.enumerateDevices(); } catch (e) {}
  }
  var labelsRevealed = mediaDevs.some(function(d){ return d.label; });

  // If we have devices but no labels yet, show an "Authorize" panel first.
  // Browsers hide labels until at least one getUserMedia call has been
  // accepted on this origin (anti-fingerprinting).
  if (hasMedia && mediaDevs.length > 0 && !labelsRevealed) {
    var perm = document.createElement('div');
    perm.className = 'periph-perm';
    perm.innerHTML = '<p>' + (T.periph_perm_msg || 'Allow camera and microphone to detect your devices.') + '</p>'
      + '<button onclick="requestMediaPermission()">' + (T.periph_perm_btn || 'Authorize') + '</button>';
    list.appendChild(perm);
  }

  // Media devices (cam/mic/speakers)
  for (var i = 0; i < mediaDevs.length; i++) {
    var d = mediaDevs[i];
    if (!d.label && !labelsRevealed) continue; // skip unlabeled until perm
    var label = d.label || (d.kind + ' ' + (i+1));
    appendPeripheralItem('media-' + d.deviceId, d.kind, label);
  }

  // Already-paired WebUSB devices
  if (navigator.usb) {
    try {
      var usbDevs = await navigator.usb.getDevices();
      for (var j = 0; j < usbDevs.length; j++) {
        var u = usbDevs[j];
        var name = (u.productName || 'USB device') + (u.manufacturerName ? ' (' + u.manufacturerName + ')' : '');
        appendPeripheralItem('usb-' + u.vendorId + '-' + u.productId + '-' + (u.serialNumber || j), 'usb', name);
      }
    } catch (e) {}
  }

  // Already-paired WebHID devices
  if (navigator.hid) {
    try {
      var hidDevs = await navigator.hid.getDevices();
      for (var k = 0; k < hidDevs.length; k++) {
        var h = hidDevs[k];
        appendPeripheralItem('hid-' + h.vendorId + '-' + h.productId + '-' + k, 'hid', h.productName || ('HID device ' + (k+1)));
      }
    } catch (e) {}
  }

  // Connected gamepads
  try {
    var gps = navigator.getGamepads ? navigator.getGamepads() : [];
    for (var g = 0; g < gps.length; g++) {
      if (gps[g]) appendPeripheralItem('gp-' + gps[g].index, 'gamepad', gps[g].id);
    }
  } catch (e) {}

  // Already-paired Web Bluetooth devices (Chromium only; getDevices() needs
  // the chrome://flags experimental-web-platform-features to be on for some
  // versions, gracefully no-op otherwise).
  if (navigator.bluetooth && navigator.bluetooth.getDevices) {
    try {
      var btDevs = await navigator.bluetooth.getDevices();
      for (var b = 0; b < btDevs.length; b++) {
        var bt = btDevs[b];
        appendPeripheralItem('bt-' + bt.id, 'bluetooth', (bt.name || 'Bluetooth device') + ' (BT)');
      }
    } catch (e) {}
  }

  // Already-paired Web Serial ports
  if (navigator.serial) {
    try {
      var ports = await navigator.serial.getPorts();
      for (var p = 0; p < ports.length; p++) {
        var info = ports[p].getInfo();
        var sName = 'Serial port' + (info.usbVendorId ? ' (' + info.usbVendorId.toString(16) + ':' + (info.usbProductId || 0).toString(16) + ')' : '');
        appendPeripheralItem('serial-' + p, 'serial', sName);
      }
    } catch (e) {}
  }

  // Web MIDI — once permission granted, list all inputs/outputs
  if (peripheralsMidiAccess) {
    peripheralsMidiAccess.inputs.forEach(function(inp) {
      appendPeripheralItem('midi-in-' + inp.id, 'midi', (inp.name || 'MIDI input') + ' (in)');
    });
    peripheralsMidiAccess.outputs.forEach(function(outp) {
      appendPeripheralItem('midi-out-' + outp.id, 'midi', (outp.name || 'MIDI output') + ' (out)');
    });
  }

  // Folders the user picked this session (File System Access)
  // Screens shared this session (getDisplayMedia)
  // — both tracked manually in peripheralsExtras since neither API has a
  //   "list previously granted" call.
  for (var x = 0; x < peripheralsExtras.length; x++) {
    var ex = peripheralsExtras[x];
    appendPeripheralItem(ex.id, ex.kind, ex.label);
  }

  // Geolocation + NFC — single toggle items (no enumeration possible)
  if (navigator.geolocation) {
    var geoId = 'geo-share';
    if (peripheralsGeoEnabled) peripheralsEnabled[geoId] = { kind: 'geolocation', label: 'Geolocation' };
    appendPeripheralItem(geoId, 'geolocation', T.periph_geolocation || 'Share my location with the VM');
  }
  if ('NDEFReader' in window) {
    var nfcId = 'nfc-scan';
    if (peripheralsNfcEnabled) peripheralsEnabled[nfcId] = { kind: 'nfc', label: 'NFC' };
    appendPeripheralItem(nfcId, 'nfc', T.periph_nfc || 'Forward NFC tags to the VM');
  }

  // Empty state
  if (!list.children.length) {
    var empty = document.createElement('div');
    empty.className = 'periph-empty';
    empty.setAttribute('data-t', 'periph_empty');
    empty.textContent = T.periph_empty || 'No devices detected.';
    list.appendChild(empty);
  }

  // Footer: one button per "addable" API. Hidden if the API isn't supported
  // by this browser (so Firefox sees almost nothing, Chrome sees everything).
  var addRow = document.getElementById('periph-add-row');
  addRow.innerHTML = '';
  var addBtns = [];
  if (navigator.usb)        addBtns.push({fn:'addUsbPeripheral()',       label: T.periph_add_usb       || '+ USB'});
  if (navigator.bluetooth && navigator.bluetooth.requestDevice)
                            addBtns.push({fn:'addBluetoothPeripheral()', label: T.periph_add_bluetooth || '+ Bluetooth'});
  if (navigator.serial)     addBtns.push({fn:'addSerialPeripheral()',    label: T.periph_add_serial    || '+ Serial'});
  if (window.showDirectoryPicker)
                            addBtns.push({fn:'addFolderPeripheral()',    label: T.periph_add_folder    || '+ Folder'});
  if (navigator.mediaDevices && navigator.mediaDevices.getDisplayMedia)
                            addBtns.push({fn:'addScreenPeripheral()',    label: T.periph_add_screen    || '+ Screen'});
  if (navigator.requestMIDIAccess && !peripheralsMidiAccess)
                            addBtns.push({fn:'addMidiPeripheral()',      label: T.periph_add_midi      || '+ MIDI'});
  for (var ab = 0; ab < addBtns.length; ab++) {
    var b = document.createElement('button');
    b.setAttribute('onclick', addBtns[ab].fn);
    b.textContent = addBtns[ab].label;
    addRow.appendChild(b);
  }
}

function appendPeripheralItem(id, kind, label) {
  var list = document.getElementById('periph-list');
  var div = document.createElement('div');
  div.className = 'periph-item';
  var checked = peripheralsEnabled[id] ? 'checked' : '';
  div.innerHTML = '<span class="periph-label" title="' + label.replace(/"/g, '&quot;') + '">' + label + '</span>'
    + '<label class="periph-toggle"><input type="checkbox" ' + checked + '><span class="periph-slider"></span></label>';
  div.querySelector('input').addEventListener('change', function(e) {
    togglePeripheralRedirect(id, kind, label, e.target.checked);
  });
  list.appendChild(div);
}

async function requestMediaPermission() {
  try {
    var s = await navigator.mediaDevices.getUserMedia({ audio: true, video: true });
    s.getTracks().forEach(function(t){ t.stop(); });
  } catch (e) {
    // Partial ok: try audio-only then video-only.
    try { var a = await navigator.mediaDevices.getUserMedia({ audio: true }); a.getTracks().forEach(function(t){t.stop();}); } catch (_) {}
    try { var v = await navigator.mediaDevices.getUserMedia({ video: true }); v.getTracks().forEach(function(t){t.stop();}); } catch (_) {}
  }
  peripheralsPermsAsked = true;
  await refreshPeripheralsList();
}

async function addUsbPeripheral() {
  if (!navigator.usb) return;
  try {
    await navigator.usb.requestDevice({ filters: [] });
    await refreshPeripheralsList();
  } catch (e) {
    // user cancelled — ignore
  }
}

async function addBluetoothPeripheral() {
  if (!navigator.bluetooth) return;
  try {
    await navigator.bluetooth.requestDevice({ acceptAllDevices: true });
    await refreshPeripheralsList();
  } catch (e) {}
}

async function addSerialPeripheral() {
  if (!navigator.serial) return;
  try {
    await navigator.serial.requestPort({});
    await refreshPeripheralsList();
  } catch (e) {}
}

async function addFolderPeripheral() {
  if (!window.showDirectoryPicker) return;
  try {
    var handle = await window.showDirectoryPicker({ mode: 'readwrite' });
    var id = 'fs-' + Date.now() + '-' + Math.random().toString(36).slice(2, 7);
    peripheralsExtras.push({ id: id, kind: 'folder', label: handle.name + ' /', handle: handle });
    await refreshPeripheralsList();
  } catch (e) {}
}

async function addScreenPeripheral() {
  if (!navigator.mediaDevices || !navigator.mediaDevices.getDisplayMedia) return;
  try {
    var stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true });
    var track = stream.getVideoTracks()[0];
    var label = track ? track.label : 'Shared screen';
    var id = 'screen-' + Date.now();
    peripheralsExtras.push({ id: id, kind: 'screen', label: label, handle: stream });
    // Keep stream alive — actual forwarding wired in Phase 1.1. For now we
    // just tag it as "enabled" so the toggle reflects state.
    peripheralsEnabled[id] = { kind: 'screen', label: label };
    await refreshPeripheralsList();
  } catch (e) {}
}

async function addMidiPeripheral() {
  if (!navigator.requestMIDIAccess) return;
  try {
    peripheralsMidiAccess = await navigator.requestMIDIAccess({ sysex: false });
    await refreshPeripheralsList();
  } catch (e) {}
}

function togglePeripheralRedirect(id, kind, label, on) {
  if (on) peripheralsEnabled[id] = { kind: kind, label: label };
  else delete peripheralsEnabled[id];

  // Some kinds need a one-time permission grant on first enable.
  if (kind === 'geolocation' && on && !peripheralsGeoEnabled) {
    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(function() {
        peripheralsGeoEnabled = true;
        // Permission is now granted; subsequent enables will just relay.
      }, function() {
        // User denied — uncheck the toggle.
        delete peripheralsEnabled[id];
        refreshPeripheralsList();
      }, { timeout: 10000 });
    }
  }
  if (kind === 'nfc' && on && !peripheralsNfcEnabled) {
    if ('NDEFReader' in window) {
      try {
        var reader = new NDEFReader();
        reader.scan().then(function() { peripheralsNfcEnabled = true; })
                     .catch(function() { delete peripheralsEnabled[id]; refreshPeripheralsList(); });
      } catch (e) { delete peripheralsEnabled[id]; refreshPeripheralsList(); }
    }
  }

  // Notify the agent so it can prepare a relay channel. The actual stream
  // forwarding (Phase 1.1) will piggyback on this signal.
  console.log('[peripherals] ' + JSON.stringify({ type: on ? 'enable' : 'disable', id: id, kind: kind, label: label }));
}

function changeSetting(name, value) {
  settings[name] = value;
  saveSettings();
  clearTimeout(reconnectTimer);
  reconnectTimer = setTimeout(reconnectWithSettings, 800);
}

var isReconnecting = false;
function reconnectWithSettings() {
  isReconnecting = true;
  var toast = document.getElementById('reconnect-toast');
  toast.classList.add('show');
  // Send new params to agent
  fetch('/sessions/' + SID + '/settings', {
    method: 'POST',
    headers: {'Content-Type': 'application/json', 'Authorization': 'Bearer ' + SESSION_TOKEN},
    body: JSON.stringify(settings)
  }).then(function() {
    // Disconnect current session
    if (guac) { guac.disconnect(); }
    // Clear display
    var display = document.getElementById('display');
    while (display.firstChild) display.removeChild(display.firstChild);
    // Reconnect
    setTimeout(function() {
      connect();
      toast.classList.remove('show');
    }, 500);
  });
}

loadSettings();

// T_CTRL_ALT_DEL
function sendCAD() {
  if (!guac) return;
  guac.sendKeyEvent(1, 0xFFE3); // Ctrl
  guac.sendKeyEvent(1, 0xFFE9); // Alt
  guac.sendKeyEvent(1, 0xFFFF); // Delete
  guac.sendKeyEvent(0, 0xFFFF);
  guac.sendKeyEvent(0, 0xFFE9);
  guac.sendKeyEvent(0, 0xFFE3);
}

// Virtual keyboard (mobile) — focuses a hidden textarea to trigger native keyboard
var vkbOpen = false;
function toggleVirtualKeyboard() {
  var input = document.getElementById('virtual-kb-input');
  if (vkbOpen) {
    input.blur();
    vkbOpen = false;
  } else {
    input.focus();
    vkbOpen = true;
  }
}

// Clipboard paste
function doClip() {
  if (!guac || !navigator.clipboard) return;
  navigator.clipboard.readText().then(function(text) {
    var stream = guac.createClipboardStream('text/plain');
    var writer = new Guacamole.StringWriter(stream);
    writer.sendText(text);
    writer.sendEnd();
  });
}

// Client-side screenshot: capture display canvas and upload to agent every 10s
setInterval(function() {
  if (!guac || !connected) return;
  try {
    var display = guac.getDisplay();
    var canvas = display.flatten();
    canvas.toBlob(function(blob) {
      if (blob && blob.size > 100) {
        fetch('/api/screenshot/' + SID + '/upload', { method: 'POST', body: blob, headers: { 'Content-Type': 'image/png', 'Authorization': 'Bearer ' + SESSION_TOKEN } });
      }
    }, 'image/png');
  } catch(e) {}
}, 10000);

// Stats
setInterval(function() {
  if (!statsOn) return;
  var el = document.getElementById('stats');
  el.innerHTML =
    '<b style="color:#7096ff">Guacamole</b><br>' +
    'Connected: ' + (connected ? '<span style="color:#10b981">Yes</span>' : '<span style="color:#ef4444">No</span>') + '<br>' +
    'Protocol: ' + PROTO.toUpperCase() + '<br>' +
    'Transport: WebSocket<br>' +
    'Session: ' + SID.substring(0, 8) + '...<br>' +
    'Agent: AGENT_NAME_PLACEHOLDER';
}, 2000);

// Apply saved settings to agent before first connect
(function() {
  var saved = {};
  try { saved = JSON.parse(localStorage.getItem(PREFS_KEY)) || {}; } catch(e) {}
  var hasCustom = false;
  SETTING_NAMES.forEach(function(n) { if (saved[n] !== undefined) hasCustom = true; });
  if (hasCustom) {
    fetch('/sessions/' + SID + '/settings', {
      method: 'POST',
      headers: {'Content-Type': 'application/json', 'Authorization': 'Bearer ' + SESSION_TOKEN},
      body: JSON.stringify(saved)
    }).then(function() { connect(); });
  } else {
    connect();
  }
})();
</script>
</body>
</html>`

	// Escape all user-controlled values to prevent XSS
	esc := htmlpkg.EscapeString
	html = strings.ReplaceAll(html, "SESSION_ID_PLACEHOLDER", esc(sessionID))
	html = strings.ReplaceAll(html, "AGENT_NAME_PLACEHOLDER", esc(agentName))
	html = strings.ReplaceAll(html, "CONTROL_PLANE_PLACEHOLDER", esc(controlPlaneURL))
	html = strings.ReplaceAll(html, "PROTOCOL_PLACEHOLDER", esc(protocol))
	html = strings.ReplaceAll(html, "LANG_PLACEHOLDER", esc(lang))
	if defaultSettings == "" || defaultSettings == "{}" {
		html = strings.ReplaceAll(html, "__DFCFG__", "{}")
	} else {
		html = strings.ReplaceAll(html, "__DFCFG__", defaultSettings)
	}
	if shadow {
		html = strings.ReplaceAll(html, "__SHADOW__", "true")
	} else {
		html = strings.ReplaceAll(html, "__SHADOW__", "false")
	}

	// i18n: build JSON object for JS translations + replace HTML placeholders
	tr := map[string]map[string]string{
		"en": {"fullscreen": "Fullscreen", "ctrl_alt_del": "Ctrl+Alt+Del", "virtual_keyboard": "Virtual Keyboard", "remote_cursor": "Remote cursor", "upload_files": "Upload files", "upload_drop": "Drop files here or", "upload_browse": "browse", "cancel": "Cancel", "destroy_confirm_msg": "This will terminate the remote connection.", "disconnected_title": "Session disconnected", "disconnected_msg": "The remote session has been disconnected.", "reconnect": "Reconnect", "display_scale": "Display scale", "back": "Back to workspaces", "destroy": "Destroy session", "settings": "Display settings", "font_smoothing": "Font smoothing", "wallpaper": "Wallpaper", "theming": "Theming", "desktop_effects": "Desktop effects", "window_drag": "Window drag", "menu_animations": "Menu animations", "audio": "Audio", "clipboard": "Clipboard", "keyboard_layout": "Keyboard layout", "color_depth": "Color depth", "fast": "fast", "quality": "quality", "connecting": "Connecting via", "reconnecting": "Reconnecting...", "disconnected": "Disconnected.", "destroy_confirm": "Destroy this session?", "shadow_mode": "Shadow Mode", "language": "Language", "peripherals": "Peripherals", "peripherals_redirect": "Peripheral redirection", "periph_perm_msg": "Allow camera and microphone to detect your devices.", "periph_perm_btn": "Authorize", "periph_add_usb": "+ USB", "periph_add_bluetooth": "+ Bluetooth", "periph_add_serial": "+ Serial", "periph_add_folder": "+ Folder", "periph_add_screen": "+ Screen", "periph_add_midi": "+ MIDI", "periph_geolocation": "Share my location with the VM", "periph_nfc": "Forward NFC tags to the VM", "periph_empty": "No devices detected."},
		"fr": {"fullscreen": "Plein \u00e9cran", "ctrl_alt_del": "Ctrl+Alt+Suppr", "virtual_keyboard": "Clavier virtuel", "remote_cursor": "Curseur distant", "upload_files": "Envoyer des fichiers", "upload_drop": "D\u00e9posez vos fichiers ici ou", "upload_browse": "parcourir", "cancel": "Annuler", "destroy_confirm_msg": "Cela mettra fin \u00e0 la connexion distante.", "disconnected_title": "Session d\u00e9connect\u00e9e", "disconnected_msg": "La session distante a \u00e9t\u00e9 d\u00e9connect\u00e9e.", "reconnect": "Reconnecter", "display_scale": "\u00c9chelle d'affichage", "back": "Retour aux workspaces", "destroy": "D\u00e9truire la session", "settings": "Param\u00e8tres d'affichage", "font_smoothing": "Lissage des polices", "wallpaper": "Fond d'\u00e9cran", "theming": "Th\u00e8me", "desktop_effects": "Effets de bureau", "window_drag": "Glisser les fen\u00eatres", "menu_animations": "Animations menus", "audio": "Audio", "clipboard": "Presse-papiers", "keyboard_layout": "Disposition clavier", "color_depth": "Profondeur couleur", "fast": "rapide", "quality": "qualit\u00e9", "connecting": "Connexion via", "reconnecting": "Reconnexion...", "disconnected": "D\u00e9connect\u00e9.", "destroy_confirm": "D\u00e9truire cette session ?", "shadow_mode": "Mode observation", "language": "Langue", "peripherals": "P\u00e9riph\u00e9riques", "peripherals_redirect": "Redirection des p\u00e9riph\u00e9riques", "periph_perm_msg": "Autorisez l'acc\u00e8s cam\u00e9ra et micro pour d\u00e9tecter vos p\u00e9riph\u00e9riques.", "periph_perm_btn": "Autoriser", "periph_add_usb": "+ USB", "periph_add_bluetooth": "+ Bluetooth", "periph_add_serial": "+ S\u00e9rie", "periph_add_folder": "+ Dossier", "periph_add_screen": "+ \u00c9cran", "periph_add_midi": "+ MIDI", "periph_geolocation": "Partager ma position avec la VM", "periph_nfc": "Transmettre les tags NFC \u00e0 la VM", "periph_empty": "Aucun p\u00e9riph\u00e9rique d\u00e9tect\u00e9."},
		"es": {"fullscreen": "Pantalla completa", "ctrl_alt_del": "Ctrl+Alt+Supr", "virtual_keyboard": "Teclado virtual", "remote_cursor": "Cursor remoto", "upload_files": "Subir archivos", "upload_drop": "Suelta archivos aqu\u00ed o", "upload_browse": "examinar", "cancel": "Cancelar", "destroy_confirm_msg": "Esto terminar\u00e1 la conexi\u00f3n remota.", "disconnected_title": "Sesi\u00f3n desconectada", "disconnected_msg": "La sesi\u00f3n remota se ha desconectado.", "reconnect": "Reconectar", "display_scale": "Escala de pantalla", "back": "Volver a workspaces", "destroy": "Destruir sesi\u00f3n", "settings": "Ajustes de pantalla", "font_smoothing": "Suavizado de fuentes", "wallpaper": "Fondo de pantalla", "theming": "Tema", "desktop_effects": "Efectos de escritorio", "window_drag": "Arrastrar ventanas", "menu_animations": "Animaciones de men\u00fa", "audio": "Audio", "clipboard": "Portapapeles", "keyboard_layout": "Disposici\u00f3n del teclado", "color_depth": "Profundidad de color", "fast": "r\u00e1pido", "quality": "calidad", "connecting": "Conectando via", "reconnecting": "Reconectando...", "disconnected": "Desconectado.", "destroy_confirm": "\u00bfDestruir esta sesi\u00f3n?", "shadow_mode": "Modo sombra", "language": "Idioma", "peripherals": "Perif\u00e9ricos", "peripherals_redirect": "Redirecci\u00f3n de perif\u00e9ricos", "periph_perm_msg": "Permite el acceso a c\u00e1mara y micr\u00f3fono para detectar tus dispositivos.", "periph_perm_btn": "Autorizar", "periph_add_usb": "+ USB", "periph_add_bluetooth": "+ Bluetooth", "periph_add_serial": "+ Serie", "periph_add_folder": "+ Carpeta", "periph_add_screen": "+ Pantalla", "periph_add_midi": "+ MIDI", "periph_geolocation": "Compartir mi ubicaci\u00f3n con la VM", "periph_nfc": "Reenviar etiquetas NFC a la VM", "periph_empty": "Ning\u00fan dispositivo detectado."},
		"de": {"fullscreen": "Vollbild", "ctrl_alt_del": "Strg+Alt+Entf", "virtual_keyboard": "Bildschirmtastatur", "remote_cursor": "Remote-Cursor", "upload_files": "Dateien hochladen", "upload_drop": "Dateien hier ablegen oder", "upload_browse": "durchsuchen", "cancel": "Abbrechen", "destroy_confirm_msg": "Die Remote-Verbindung wird getrennt.", "disconnected_title": "Sitzung getrennt", "disconnected_msg": "Die Remote-Sitzung wurde getrennt.", "reconnect": "Erneut verbinden", "display_scale": "Anzeigeskalierung", "back": "Zur\u00fcck zu Workspaces", "destroy": "Sitzung zerst\u00f6ren", "settings": "Anzeigeeinstellungen", "font_smoothing": "Schriftgl\u00e4ttung", "wallpaper": "Hintergrundbild", "theming": "Design", "desktop_effects": "Desktop-Effekte", "window_drag": "Fenster ziehen", "menu_animations": "Men\u00fc-Animationen", "audio": "Audio", "clipboard": "Zwischenablage", "keyboard_layout": "Tastaturlayout", "color_depth": "Farbtiefe", "fast": "schnell", "quality": "Qualit\u00e4t", "connecting": "Verbindung \u00fcber", "reconnecting": "Verbindung wird wiederhergestellt...", "disconnected": "Getrennt.", "destroy_confirm": "Diese Sitzung zerst\u00f6ren?", "shadow_mode": "Beobachtungsmodus", "language": "Sprache", "peripherals": "Peripherieger\u00e4te", "peripherals_redirect": "Peripherie-Umleitung", "periph_perm_msg": "Erlaube Kamera- und Mikrofonzugriff, um Ger\u00e4te zu erkennen.", "periph_perm_btn": "Erlauben", "periph_add_usb": "+ USB", "periph_add_bluetooth": "+ Bluetooth", "periph_add_serial": "+ Seriell", "periph_add_folder": "+ Ordner", "periph_add_screen": "+ Bildschirm", "periph_add_midi": "+ MIDI", "periph_geolocation": "Standort an VM senden", "periph_nfc": "NFC-Tags an VM weiterleiten", "periph_empty": "Keine Ger\u00e4te erkannt."},
	}

	// Build JSON for ALL translations (embedded in JS for runtime switching)
	allTrJSON, _ := json.Marshal(tr)
	html = strings.ReplaceAll(html, "ALL_TRANSLATIONS_PLACEHOLDER", string(allTrJSON))

	// Use the session language for initial HTML placeholder replacement
	t2 := tr["en"]
	if langTr, ok := tr[lang]; ok {
		t2 = langTr
	}
	// Replace HTML placeholders (longest keys first to avoid partial matches)
	keys := make([]string, 0, len(t2))
	for k := range t2 {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		html = strings.ReplaceAll(html, "T_"+strings.ToUpper(k), t2[k])
	}
	return html
}
