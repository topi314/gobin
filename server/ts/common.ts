import type { DocumentsMap, JwtPayload } from "./types";

const DOCUMENTS_KEY = "documents";

export const PermissionWrite = 1;
export const PermissionDelete = 2;
export const PermissionShare = 4;
export const PermissionWebhook = 8;

export function listDocuments(): DocumentsMap {
	const raw = localStorage.getItem(DOCUMENTS_KEY);
	if (!raw) {
		return {};
	}
	try {
		return JSON.parse(raw) as DocumentsMap;
	} catch {
		return {};
	}
}

export function getToken(key: string): string {
	if (!key) {
		return "";
	}
	return listDocuments()[key] ?? "";
}

export function setToken(key: string, token: string): void {
	const documents = listDocuments();
	documents[key] = token;
	localStorage.setItem(DOCUMENTS_KEY, JSON.stringify(documents));
}

export function deleteToken(key: string): void {
	const documents = listDocuments();
	delete documents[key];
	localStorage.setItem(DOCUMENTS_KEY, JSON.stringify(documents));
}

export function deleteTokens(keys: string[]): void {
	if (keys.length === 0) {
		return;
	}
	const documents = listDocuments();
	for (const key of keys) {
		delete documents[key];
	}
	localStorage.setItem(DOCUMENTS_KEY, JSON.stringify(documents));
}

export function decodeJwtPayload(token: string): JwtPayload | null {
	const parts = token.split(".");
	if (parts.length !== 3) {
		return null;
	}
	try {
		const payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
		return JSON.parse(atob(payload)) as JwtPayload;
	} catch {
		return null;
	}
}

export function hasPermission(token: string, permission: number): boolean {
	const claims = decodeJwtPayload(token);
	if (!claims?.pms) {
		return false;
	}
	return (claims.pms & permission) === permission;
}

export function showErrorPopup(message?: string): void {
	const popup = document.getElementById("error-popup");
	if (!popup) {
		return;
	}
	popup.style.display = "block";
	popup.innerText = message || "Something went wrong.";
	setTimeout(() => {
		popup.style.display = "none";
	}, 5000);
}

export function updateFaviconStyle(matches: boolean): void {
	const faviconElement = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
	if (!faviconElement) {
		return;
	}
	faviconElement.href = matches ? "/assets/favicon.png" : "/assets/favicon-light.png";
}

export function setCookie(
	name: string,
	value: string,
	options: Record<string, string | boolean | Date> = {},
): void {
	const opts: Record<string, string | boolean | Date> = {
		path: "/",
		sameSite: "strict",
		...options,
	};

	if (opts.expires instanceof Date) {
		opts.expires = opts.expires.toUTCString();
	}

	let updatedCookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)}`;
	for (const optionKey in opts) {
		updatedCookie += `; ${optionKey}`;
		const optionValue = opts[optionKey];
		if (optionValue !== true) {
			updatedCookie += `=${optionValue}`;
		}
	}
	document.cookie = updatedCookie;
}
