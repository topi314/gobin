import {
	PermissionDelete,
	PermissionShare,
	decodeJwtPayload,
	deleteToken,
	getToken,
	hasPermission,
	listDocuments,
	showErrorPopup,
	updateFaviconStyle,
} from "./common";

interface OverviewEntry {
	id: string;
	token: string;
	iat: number;
}

let shareDocumentID = "";

function formatTime(iat: number): string {
	if (!iat) {
		return "Unknown date";
	}
	return new Date(iat * 1000).toLocaleString();
}

function documentURL(id: string): string {
	return `${window.location.origin}/${id}`;
}

function iconButton(id: string, title: string): HTMLButtonElement {
	const button = document.createElement("button");
	button.type = "button";
	button.className = `icon-btn ${id}`;
	button.title = title;
	button.setAttribute("aria-label", title);
	return button;
}

function shareDialog(): HTMLDialogElement {
	return document.getElementById("share-dialog") as HTMLDialogElement;
}

function selectedPermissions(): string[] {
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
	return permissions;
}

async function deleteDocument(id: string, token: string): Promise<boolean> {
	const response = await fetch(`/documents/${id}`, {
		method: "DELETE",
		headers: { Authorization: `Bearer ${token}` },
	});
	if (response.status === 204 || response.ok) {
		return true;
	}
	let message = response.statusText;
	try {
		const body = (await response.json()) as { message?: string };
		message = body.message || message;
	} catch {
		// ignore
	}
	showErrorPopup(message);
	return false;
}

async function openShare(id: string, token: string): Promise<void> {
	if (!hasPermission(token, PermissionShare)) {
		await navigator.clipboard.writeText(documentURL(id));
		return;
	}

	shareDocumentID = id;
	(document.getElementById("share-permissions-write") as HTMLInputElement).checked = false;
	(document.getElementById("share-permissions-delete") as HTMLInputElement).checked = false;
	(document.getElementById("share-permissions-share") as HTMLInputElement).checked = false;
	(document.getElementById("share-permissions-webhook") as HTMLInputElement).checked = false;
	shareDialog().showModal();
}

function bindShareDialog(): void {
	document.getElementById("share-dialog-close")?.addEventListener("click", () => {
		shareDialog().close();
		shareDocumentID = "";
	});

	document.getElementById("share-copy")?.addEventListener("click", async () => {
		const id = shareDocumentID;
		if (!id) {
			return;
		}

		const permissions = selectedPermissions();
		if (permissions.length === 0) {
			await navigator.clipboard.writeText(documentURL(id));
			shareDialog().close();
			shareDocumentID = "";
			return;
		}

		const token = getToken(id);
		const response = await fetch(`/documents/${id}/share`, {
			method: "POST",
			body: JSON.stringify({ permissions }),
			headers: {
				"Content-Type": "application/json",
				Authorization: `Bearer ${token}`,
			},
		});

		if (!response.ok) {
			let message = response.statusText;
			try {
				const body = (await response.json()) as { message?: string };
				message = body.message || message;
			} catch {
				// ignore
			}
			showErrorPopup(message);
			console.error("error sharing document:", response);
			return;
		}

		const body = (await response.json()) as { token?: string };
		await navigator.clipboard.writeText(`${documentURL(id)}?token=${body.token}`);
		shareDialog().close();
		shareDocumentID = "";
	});
}

function renderList(): void {
	const list = document.getElementById("document-list");
	const empty = document.getElementById("document-list-empty");
	if (!list || !empty) {
		return;
	}

	const documents = listDocuments();
	const entries: OverviewEntry[] = Object.entries(documents).map(([id, token]) => {
		const payload = decodeJwtPayload(token);
		return { id, token, iat: payload?.iat ?? 0 };
	});
	entries.sort((a, b) => b.iat - a.iat);

	list.replaceChildren();
	if (entries.length === 0) {
		empty.style.display = "block";
		return;
	}
	empty.style.display = "none";

	for (const entry of entries) {
		const li = document.createElement("li");

		const link = document.createElement("a");
		link.className = "doc-id";
		link.href = `/${entry.id}`;
		link.textContent = entry.id;

		const time = document.createElement("span");
		time.className = "doc-time";
		time.textContent = formatTime(entry.iat);

		const actions = document.createElement("div");
		actions.className = "doc-actions";

		const copy = iconButton("copy", "Copy link");
		copy.addEventListener("click", async () => {
			await navigator.clipboard.writeText(documentURL(entry.id));
		});

		const raw = iconButton("raw", "Raw");
		raw.addEventListener("click", () => {
			window.open(`/raw/${entry.id}`, "_blank")?.focus();
		});

		const share = iconButton("share", "Share");
		share.addEventListener("click", () => {
			void openShare(entry.id, entry.token);
		});

		const del = iconButton("delete", "Delete");
		const canDelete = hasPermission(entry.token || getToken(entry.id), PermissionDelete);
		del.disabled = !canDelete;
		del.addEventListener("click", async () => {
			if (del.disabled) {
				return;
			}
			if (!window.confirm("Are you sure you want to delete this document? This action cannot be undone.")) {
				return;
			}
			del.classList.add("loading");
			const ok = await deleteDocument(entry.id, entry.token);
			del.classList.remove("loading");
			if (!ok) {
				return;
			}
			deleteToken(entry.id);
			renderList();
		});

		const forget = iconButton("doc-remove", "Remove from list");
		forget.addEventListener("click", () => {
			deleteToken(entry.id);
			renderList();
		});

		actions.append(del, copy, raw, share, forget);
		li.append(link, time, actions);
		list.append(li);
	}
}

document.addEventListener("DOMContentLoaded", () => {
	updateFaviconStyle(window.matchMedia("(prefers-color-scheme: dark)").matches);
	bindShareDialog();
	renderList();
});

window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (event) => {
	updateFaviconStyle(event.matches);
});
