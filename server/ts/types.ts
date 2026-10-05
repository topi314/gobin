export type DocumentMode = "edit" | "view";

export interface DocumentFile {
	name: string;
	content: string;
	formatted: string;
	language: string;
}

export interface DocumentState {
	key: string;
	version: number;
	mode: DocumentMode;
	files: DocumentFile[];
	current_file: number;
	expire_in: number;
}

export interface DocumentResponse {
	key: string;
	version: number;
	version_time?: string;
	version_label?: string;
	token?: string;
	files: DocumentFile[];
	message?: string;
}

export interface JwtPayload {
	sub?: string;
	iat?: number;
	pms?: number;
}

export interface StoredDocument {
	id: string;
	token: string;
}

export type Permission = 1 | 2 | 4 | 8;

export interface CookieOptions {
	path?: string;
	sameSite?: string;
	expires?: Date | string;
	[key: string]: string | boolean | Date | undefined;
}
