export type DocMeta = {
	titleKey: string;
	descriptionKey: string;
	order: number;
};

export type DocSection =
	| { type: 'heading'; key: string }
	| { type: 'paragraph'; key: string }
	| { type: 'list'; keys: string[] }
	| { type: 'code'; code: string; language?: string }
	| { type: 'mermaid'; code: string };

export type DocRecord = DocMeta & {
	slug: string;
	sections: DocSection[];
};
