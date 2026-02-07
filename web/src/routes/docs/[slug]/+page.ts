import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import { getDocBySlug, getDocList } from '$lib/docs';

export const load: PageLoad = ({ params }) => {
	const doc = getDocBySlug(params.slug);
	if (!doc) {
		throw error(404, 'Doc not found');
	}
	return {
		doc,
		docs: getDocList()
	};
};
