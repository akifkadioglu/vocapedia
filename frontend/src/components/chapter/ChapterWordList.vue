<template>
    <div>
        <div class="flex justify-center">
            <input v-model="search" type="text" :placeholder="$t('chapter.search_chapter',{name:props.response.chapter?.title})" class="w-full p-3 border rounded-lg shadow-sm outline-none transition-all 
             bg-white text-zinc-900  border-none
             max-w-160 
             dark:bg-zinc-800 dark:text-white " />
        </div>
        <div v-auto-animate>
            <div :id="item.words[0].word.replace(/\s+/g, '-')" v-for="item in filteredList" :key="item.id"
                class="flex justify-center py-2.5">
                <div class="max-w-160 w-full ">
                    <div v-auto-animate class="card transition duration-200  hover:shadow p-4">
                        <div class="flex justify-between items-center space-x-5">
                            <div class="font-bold text-xl capitalize ">{{ item.words[0].word }}</div>
                            <span
                                class="more-than-word bg-blue-200 dark:bg-blue-800 px-2 rounded-full text-blue-800 dark:text-blue-200">
                                {{ $t('shared.word_types.' + item.type) }}
                            </span>
                        </div>
                        <div class="font-light pt-5">{{ item.words[0].description }}</div>

                        <div class="more-than-word">
                            <div v-if="(item.words[0].examples ?? []).length > 0">
                                <div v-for="example in item.words[0].examples"
                                    class="p-5 space-x-2 flex items-end font-light text-sm">
                                    <mdicon name="arrow-right" size="20" />
                                    <span>{{ example }}</span>
                                </div>
                            </div>
                            <div class="text-sm text-end text-sky-900 dark:text-sky-200 "> {{
                                getLangByCode(item.words[0].lang).name }}
                            </div>
                            <hr class="border-t-2 border-zinc-200 dark:border-zinc-800 my-4 opacity-50">
                        </div>

                        <div :id="sub.word.replace(/\s+/g, '-')" class="more-than-word" v-for="(sub, i) in item.words.slice(1)" :key="i">
                            <div class="font-bold text-xl capitalize py-5">{{ sub.word }}</div>
                            <div class="font-light">{{ sub.description }}</div>
                            <div v-if="(sub.examples ?? []).length > 0">
                                <div v-for="example in sub.examples"
                                    class="space-x-2 p-5 flex items-end font-light text-sm">
                                    <mdicon name="arrow-right" size="20" />
                                    <span>{{ example }}</span>
                                </div>
                            </div>
                            <div class="text-sm text-end text-sky-900 dark:text-sky-200 "> {{
                                getLangByCode(sub.lang).name }}</div>
                            <hr v-if="i < item.words.length - 2"
                                class="border-t-2 border-zinc-200 dark:border-zinc-800 my-4 opacity-50">

                        </div>

                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { getLangByCode } from '@/utils/language/languages';
import { computed, ref, onMounted } from 'vue';
import { useRoute } from "vue-router"
const search = ref('');
const route = useRoute()
const filteredList = computed(() => {
    return props.response.chapter?.word_bases?.filter(x => x.words[0].word.toLowerCase().includes(search.value.toLowerCase()))
});

const props = defineProps({
    response: {
        type: Object,
        required: true,
    }
})

onMounted(() => {
    if (window.location.hash && route.query.variant != 'tutorial') {
        const decodedHash = decodeURIComponent(window.location.hash);

        const validHash = decodedHash.replace(/\s+/g, '-');

        const element = document.querySelector(validHash);

        if (element) {
            element.scrollIntoView({ behavior: 'smooth' });
        }
    }
});
</script>
<style scoped>
.more-than-word {
    opacity: 0;
    visibility: hidden;
    height: 0;
    overflow: hidden;
    transition: opacity 0.3s ease, visibility 0s linear 0.3s, height 0.3s ease;
}

.card:hover .more-than-word {
    opacity: 1;
    visibility: visible;
    height: auto;
    transition: opacity 0.3s ease, visibility 0s linear 0s, height 0.3s ease;
}
</style>
