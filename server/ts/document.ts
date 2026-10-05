import {
	PermissionDelete,
	PermissionShare,
	PermissionWrite,
	deleteToken,
	getToken,
	hasPermission,
	setCookie,
	setToken,
	showErrorPopup,
	updateFaviconStyle,
} from "./common";
import type { DocumentFile, DocumentResponse, DocumentState } from "./types";

function getState(): DocumentState {
	return window.history.state as DocumentState;
}

function getURL(state: DocumentState): string {
	const url = new URL(window.location.href);
	if (state.files.length > 1) {
		url.searchParams.set("file", state.files[state.current_file].name);
	} else {
		url.searchParams.delete("file");
	}
	url.pathname = `/${state.key}${state.version !== 0 ? `/${state.version}` : ""}`;
	return url.toString();
}

function setState(state: DocumentState): void {
	window.history.replaceState(state, "", getURL(state));
}

function addState(state: DocumentState): void {
	window.history.pushState(state, "", getURL(state));
}

function syncFileContent(target: HTMLTextAreaElement): void {
	const state = getState();
	state.files[state.current_file].content = target.value;
	updateButtons(state);
	setState(state);
}

async function parseJSONResponse(response: Response): Promise<DocumentResponse> {
	let body: unknown = await response.text();
	try {
		body = JSON.parse(body as string);
	} catch {
		body = { message: body };
	}
	return body as DocumentResponse;
}

async function saveDocument(
	key: string,
	expire: number | undefined,
	files: DocumentFile[],
): Promise<DocumentResponse | undefined> {
	const data = new FormData();
	for (const [i, file] of files.entries()) {
		const blob = new Blob([file.content], { type: file.language });
		data.append(`file-${i}`, blob, file.name);
	}

	const headers: Record<string, string> = {};
	const token = getToken(key);
	if (token) {
		headers.Authorization = `Bearer ${token}`;
	}

	if (expire) {
		try {
			headers.Expires = new Date(Date.now() + expire * 60 * 60 * 1000).toISOString();
		} catch {
			showErrorPopup("Invalid expiration date");
			return;
		}
	}

	const response = await fetch(`/documents/${key}?formatter=html`, {
		body: data,
		method: key !== "" ? "PATCH" : "POST",
		headers,
	});

	const body = await parseJSONResponse(response);
	if (!response.ok) {
		showErrorPopup(body.message || response.statusText);
		console.error("error saving document:", response);
		return;
	}
	return body;
}

async function fetchDocument(key: string, version: number): Promise<DocumentResponse | undefined> {
	const response = await fetch(
		`/documents/${key}${version !== 0 ? `/versions/${version}` : ""}?formatter=html`,
		{ method: "GET" },
	);
	const body = await parseJSONResponse(response);
	if (!response.ok) {
		showErrorPopup(body.message || response.statusText);
		console.error("error fetching document version:", response);
		return;
	}
	return body;
}

async function fetchDocumentFile(
	key: string,
	version: number,
	file: string,
	language: string,
): Promise<DocumentFile | undefined> {
	const response = await fetch(
		`/documents/${key}${version !== 0 ? `/versions/${version}` : ""}/files/${file}?formatter=html&language=${language}`,
		{ method: "GET" },
	);
	const body = await parseJSONResponse(response);
	if (!response.ok) {
		showErrorPopup(body.message || response.statusText);
		console.error("error fetching document file:", response);
		return;
	}
	return body as unknown as DocumentFile;
}

async function deleteDocument(key: string, token: string): Promise<void> {
	const response = await fetch(`/documents/${key}`, {
		method: "DELETE",
		headers: { Authorization: `Bearer ${token}` },
	});

	if (response.status === 204) {
		return;
	}

	const body = await parseJSONResponse(response);
	if (!response.ok) {
		showErrorPopup(body.message || response.statusText);
		console.error("error deleting document:", response);
	}
}

function updateVersionSelect(currentIndex: number): void {
	const versionElement = document.getElementById("version") as HTMLSelectElement;
	for (let i = 0; i < versionElement.options.length; i++) {
		const element = versionElement.options.item(i);
		if (!element) {
			continue;
		}
		if (element.innerText.endsWith(" (current)")) {
			element.innerText = element.innerText.substring(0, element.innerText.length - 10);
		}
	}
	if (currentIndex !== versionElement.options.length - 1 && currentIndex !== -1) {
		const option = versionElement.options.item(currentIndex);
		if (option) {
			option.innerText += " (current)";
		}
	}
}

function updateFiles(state: DocumentState): void {
	const nodes: Node[] = [];
	for (const [i, file] of state.files.entries()) {
		const input = document.createElement("input");
		input.id = `file-${i}`;
		input.type = "radio";
		input.name = "files";
		input.value = `${i}`;
		if (i === state.current_file) {
			input.checked = true;
		}

		const label = document.createElement("label");
		label.htmlFor = `file-${i}`;
		label.innerHTML = `<span>${file.name}</span><button class="file-remove" ${state.mode === "view" ? "disabled" : ""}></button>`;

		nodes.push(input);
		nodes.push(label);
	}

	const files = document.getElementById("files");
	if (!files?.lastElementChild) {
		return;
	}
	nodes.push(files.lastElementChild);
	files.replaceChildren(...nodes);
}

function updateCode(state: DocumentState | null): void {
	if (!state) {
		return;
	}

	const codeElement = document.getElementById("code") as HTMLElement;
	const codeEditElement = document.getElementById("code-edit") as HTMLTextAreaElement;

	if (state.mode === "view") {
		codeEditElement.style.display = "none";
		codeElement.style.display = "block";
	} else {
		codeEditElement.style.display = "block";
		codeElement.style.display = "none";
	}

	const file = state.files[state.current_file];
	codeEditElement.value = file.content;
	const codeView = document.getElementById("code-view");
	if (codeView) {
		codeView.innerHTML = file.formatted;
	}
	const language = document.getElementById("language") as HTMLSelectElement;
	language.value = file.language;
}

function updateButtons(state: DocumentState): void {
	const token = getToken(state.key);
	document.title = state.key ? `gobin - ${state.key}` : "gobin";

	document.querySelectorAll<HTMLButtonElement>(".file-remove").forEach((element) => {
		element.disabled = state.mode === "view";
	});

	const fileAddButton = document.getElementById("file-add") as HTMLElement;
	const saveButton = document.getElementById("save") as HTMLButtonElement;
	const editButton = document.getElementById("edit") as HTMLButtonElement;
	const deleteButton = document.getElementById("delete") as HTMLButtonElement;
	const copyButton = document.getElementById("copy") as HTMLButtonElement;
	const rawButton = document.getElementById("raw") as HTMLButtonElement;
	const shareButton = document.getElementById("share") as HTMLButtonElement;
	const expireLabel = document.querySelector<HTMLElement>('label[for="expire"]');
	const versionSelect = document.getElementById("version") as HTMLSelectElement;
	versionSelect.disabled = versionSelect.options.length <= 1;

	if (state.mode === "view") {
		fileAddButton.style.display = "none";
		saveButton.style.display = "none";
		editButton.style.display = "block";
		deleteButton.disabled = !hasPermission(token, PermissionDelete);
		copyButton.disabled = false;
		rawButton.disabled = false;
		shareButton.disabled = false;
		if (expireLabel) {
			expireLabel.style.display = "none";
		}
		return;
	}

	fileAddButton.style.display = "block";
	saveButton.style.display = "block";
	saveButton.disabled = state.files.findIndex((file) => file.content.length > 0) === -1;
	editButton.style.display = "none";
	deleteButton.disabled = true;
	copyButton.disabled = true;
	rawButton.disabled = true;
	shareButton.disabled = true;
	if (expireLabel) {
		expireLabel.style.display = "block";
	}
}

function doKeyboardAction(event: KeyboardEvent, elementId: string): void {
	event.preventDefault();
	const element = document.getElementById(elementId) as HTMLButtonElement | null;
	if (!element || element.disabled) {
		return;
	}
	element.click();
}

function bindDocumentEvents(): void {
	const filesEl = document.getElementById("files");
	const codeEdit = document.getElementById("code-edit") as HTMLTextAreaElement;
	const fileAdd = document.getElementById("file-add");

	filesEl?.addEventListener("change", (e) => {
		const state = getState();
		state.current_file = parseInt((e.target as HTMLInputElement).value, 10);
		updateCode(state);
		setState(state);
	});

	filesEl?.addEventListener("dblclick", (e) => {
		const target = e.target as HTMLElement;
		if (target.tagName.toLowerCase() !== "span") {
			return;
		}
		const state = getState();
		if (state.mode !== "edit") {
			return;
		}
		target.contentEditable = "true";
		target.focus();
	});

	filesEl?.addEventListener("focusout", (e) => {
		const target = e.target as HTMLElement;
		if (target.tagName.toLowerCase() !== "span") {
			return;
		}
		const state = getState();
		if (state.mode !== "edit") {
			return;
		}
		target.contentEditable = "false";
		if (!state.files[state.current_file].name) {
			const name = `untitled${state.current_file > 0 ? state.current_file : ""}`;
			state.files[state.current_file].name = name;
			target.innerText = name;
		}
		setState(state);
	});

	filesEl?.addEventListener("keypress", (e) => {
		const state = getState();
		if (state.mode !== "edit") {
			return;
		}
		if (e.key === "Enter") {
			e.preventDefault();
			const target = e.target as HTMLElement;
			if (target.contentEditable) {
				target.blur();
			}
		}
	});

	filesEl?.addEventListener("input", (e) => {
		const target = e.target as HTMLElement & { name?: string };
		if (target.name === "files") {
			return;
		}
		const state = getState();
		state.files[state.current_file].name = target.innerText.trim();
		setState(state);
	});

	filesEl?.addEventListener("click", (e) => {
		const target = e.target as HTMLElement;
		if (target.tagName.toLowerCase() !== "button") {
			return;
		}
		const state = getState();
		const label = target.parentElement as HTMLLabelElement;
		const index = parseInt((document.getElementById(label.htmlFor) as HTMLInputElement).value, 10);
		state.files.splice(index, 1);

		if (index === state.current_file) {
			state.current_file = 0;
		}

		if (state.files.length === 0) {
			state.files.push({
				name: "untitled",
				content: "",
				formatted: "",
				language: "auto",
			});
		}

		updateFiles(state);
		updateCode(state);
		setState(state);
	});

	fileAdd?.addEventListener("click", () => {
		const state = getState();
		const index = state.files.length;
		state.files[index] = {
			name: `untitled${index}`,
			content: "",
			formatted: "",
			language: "auto",
		};
		updateFiles(state);
		setState(state);
		document.querySelector<HTMLLabelElement>(`label[for="file-${index}"]`)?.click();
	});

	codeEdit?.addEventListener("keydown", (e) => {
		if (e.key !== "Tab" || e.shiftKey) {
			return;
		}
		e.preventDefault();
		const start = codeEdit.selectionStart;
		const end = codeEdit.selectionEnd;
		codeEdit.value = `${codeEdit.value.substring(0, start)}\t${codeEdit.value.substring(end)}`;
		codeEdit.selectionStart = codeEdit.selectionEnd = start + 1;
	});

	codeEdit?.addEventListener("input", () => {
		const state = getState();
		state.files[state.current_file].content = codeEdit.value;
		const count = state.files.reduce((total, file) => total + file.content.length, 0);
		const countEl = document.getElementById("code-edit-count");
		if (countEl) {
			countEl.innerHTML = `${count}`;
		}
		const maxElement = document.getElementById("code-edit-max");
		if (!maxElement) {
			return;
		}
		document
			.querySelector(`label[for="code-edit"]`)
			?.classList.toggle("invalid", count > Number(maxElement.innerHTML.substring(1)));
	});

	for (const eventName of ["paste", "cut", "keyup"] as const) {
		codeEdit?.addEventListener(eventName, () => syncFileContent(codeEdit));
	}

	document.getElementById("version")?.addEventListener("change", async (e) => {
		const state = getState();
		const select = e.target as HTMLSelectElement;
		let newVersion: number | string = select.value;
		if (String(newVersion) === String(state.version)) {
			return;
		}
		if (select.options.item(0)?.value === newVersion) {
			newVersion = 0;
		}

		const doc = await fetchDocument(state.key, Number(newVersion));
		if (!doc) {
			return;
		}

		state.version = doc.version;
		state.files = doc.files;
		if (state.current_file >= state.files.length) {
			state.current_file = state.files.length - 1;
		}

		updateVersionSelect(select.selectedIndex);
		updateFiles(state);
		updateCode(state);
		addState(state);
	});

	document.getElementById("style")?.addEventListener("change", (e) => {
		const style = (e.target as HTMLSelectElement).value;
		setCookie("style", style);
		const themeCssElement = document.getElementById("theme-css") as HTMLLinkElement;
		const href = new URL(themeCssElement.href);
		href.searchParams.set("style", style);
		themeCssElement.href = href.toString();
	});

	document.getElementById("expire")?.addEventListener("input", (e) => {
		const input = e.target as HTMLInputElement;
		const expireIn = parseInt(input.value, 10);
		const invalid = Number.isNaN(expireIn);
		input.classList.toggle("invalid", invalid);
		if (invalid) {
			return;
		}
		const state = getState();
		state.expire_in = expireIn;
		setState(state);
	});

	document.getElementById("language")?.addEventListener("change", async (e) => {
		const state = getState();
		const file = state.files[state.current_file];
		file.language = (e.target as HTMLSelectElement).value;
		if (state.mode === "view") {
			const updated = await fetchDocumentFile(state.key, state.version, file.name, file.language);
			if (updated) {
				state.files[state.current_file] = updated;
				updateCode(state);
			}
		}
		setState(state);
	});

	document.addEventListener("keydown", (event) => {
		const shortcuts: Record<string, string> = { s: "save", n: "new", e: "edit", d: "duplicate" };
		if (!event.ctrlKey || !(event.key in shortcuts)) {
			return;
		}
		doKeyboardAction(event, shortcuts[event.key]);
	});

	document.getElementById("edit")?.addEventListener("click", () => {
		const editButton = document.getElementById("edit") as HTMLButtonElement;
		if (editButton.disabled) {
			return;
		}
		const state = getState();
		if (!hasPermission(getToken(state.key), PermissionWrite)) {
			state.key = "";
		}
		state.mode = "edit";
		state.version = 0;
		updateCode(state);
		updateButtons(state);
		addState(state);
	});

	document.getElementById("save")?.addEventListener("click", async () => {
		const saveButton = document.getElementById("save") as HTMLButtonElement;
		if (saveButton.disabled) {
			return;
		}
		const state = getState();
		if (state.mode !== "edit") {
			return;
		}

		saveButton.classList.add("loading");
		const doc = await saveDocument(state.key, state.expire_in, state.files);
		saveButton.classList.remove("loading");
		if (!doc) {
			return;
		}

		state.key = doc.key;
		state.version = 0;
		state.files = doc.files;
		state.mode = "view";
		state.expire_in = 0;

		if (doc.token) {
			setToken(doc.key, doc.token);
		}

		const optionElement = document.createElement("option");
		optionElement.title = `${doc.version_time ?? ""}`;
		optionElement.value = String(doc.version);
		optionElement.innerText = `${doc.version_label ?? ""}`;

		updateVersionSelect(-1);
		const versionElement = document.getElementById("version") as HTMLSelectElement;
		versionElement.insertBefore(optionElement, versionElement.firstChild);
		versionElement.value = String(doc.version);

		const expire = document.getElementById("expire") as HTMLInputElement;
		expire.value = "";

		updateCode(state);
		updateButtons(state);
		addState(state);
	});

	document.getElementById("delete")?.addEventListener("click", async () => {
		const deleteButton = document.getElementById("delete") as HTMLButtonElement;
		if (deleteButton.disabled) {
			return;
		}

		const state = getState();
		const token = getToken(state.key);
		if (!token) {
			return;
		}

		if (!window.confirm("Are you sure you want to delete this document? This action cannot be undone.")) {
			return;
		}

		deleteButton.classList.add("loading");
		await deleteDocument(state.key, token);
		deleteButton.classList.remove("loading");

		deleteToken(state.key);

		state.key = "";
		state.version = 0;
		state.mode = "edit";
		state.files = [
			{
				name: "untitled",
				content: "",
				formatted: "",
				language: "auto",
			},
		];
		state.current_file = 0;

		updateCode(state);
		updateButtons(state);
		addState(state);
	});

	document.getElementById("copy")?.addEventListener("click", async () => {
		const copyButton = document.getElementById("copy") as HTMLButtonElement;
		if (copyButton.disabled) {
			return;
		}
		const state = getState();
		await navigator.clipboard.writeText(state.files[state.current_file].content);
	});

	document.getElementById("raw")?.addEventListener("click", () => {
		const rawButton = document.getElementById("raw") as HTMLButtonElement;
		if (rawButton.disabled) {
			return;
		}
		const { key, version } = getState();
		if (!key) {
			return;
		}
		window.open(`/raw/${key}${version !== 0 ? `/versions/${version}` : ""}`, "_blank")?.focus();
	});

	document.getElementById("share")?.addEventListener("click", async () => {
		const shareButton = document.getElementById("share") as HTMLButtonElement;
		if (shareButton.disabled) {
			return;
		}

		const { key } = getState();
		const token = getToken(key);
		if (!hasPermission(token, PermissionShare)) {
			await navigator.clipboard.writeText(window.location.href);
			return;
		}

		(document.getElementById("share-permissions-write") as HTMLInputElement).checked = false;
		(document.getElementById("share-permissions-delete") as HTMLInputElement).checked = false;
		(document.getElementById("share-permissions-share") as HTMLInputElement).checked = false;

		(document.getElementById("share-dialog") as HTMLDialogElement).showModal();
	});

	document.getElementById("share-dialog-close")?.addEventListener("click", () => {
		(document.getElementById("share-dialog") as HTMLDialogElement).close();
	});

	document.getElementById("share-copy")?.addEventListener("click", async () => {
		const permissions: string[] = [];
		if ((document.getElementById("share-permissions-write") as HTMLInputElement).checked) {
			permissions.push("write");
		}
		if ((document.getElementById("share-permissions-delete") as HTMLInputElement).checked) {
			permissions.push("delete");
		}
		if ((document.getElementById("share-permissions-share") as HTMLInputElement).checked) {
			permissions.push("share");
		}
		if ((document.getElementById("share-permissions-webhook") as HTMLInputElement).checked) {
			permissions.push("webhook");
		}

		if (permissions.length === 0) {
			await navigator.clipboard.writeText(window.location.href);
			(document.getElementById("share-dialog") as HTMLDialogElement).close();
			return;
		}

		const { key } = getState();
		const token = getToken(key);
		const response = await fetch(`/documents/${key}/share`, {
			method: "POST",
			body: JSON.stringify({ permissions }),
			headers: {
				"Content-Type": "application/json",
				Authorization: `Bearer ${token}`,
			},
		});

		if (!response.ok) {
			const body = await parseJSONResponse(response);
			showErrorPopup(body.message || response.statusText);
			console.error("error sharing document:", response);
			return;
		}

		const body = await parseJSONResponse(response);
		const shareUrl = `${window.location.href}?token=${body.token}`;
		await navigator.clipboard.writeText(shareUrl);
		(document.getElementById("share-dialog") as HTMLDialogElement).close();
	});
}

document.addEventListener("DOMContentLoaded", () => {
	const matches = window.matchMedia("(prefers-color-scheme: dark)").matches;
	updateFaviconStyle(matches);

	const stateEl = document.getElementById("state");
	if (!stateEl) {
		return;
	}
	const state = JSON.parse(stateEl.textContent ?? "{}") as DocumentState;

	const params = new URLSearchParams(window.location.search);
	if (params.has("token")) {
		setToken(state.key, params.get("token") ?? "");
	}

	bindDocumentEvents();
	updateButtons(state);
	setState(state);
});

window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (event) => {
	updateFaviconStyle(event.matches);
});

window.addEventListener("popstate", (event) => {
	const state = event.state as DocumentState | null;
	if (!state) {
		return;
	}
	updateFiles(state);
	updateCode(state);
	updateButtons(state);
});
