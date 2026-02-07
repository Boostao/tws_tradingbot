import type { PageLoad } from './$types';
import { getDocList } from '$lib/docs';

export const load: PageLoad = () => {
	return {
		docs: getDocList()
	};
};
