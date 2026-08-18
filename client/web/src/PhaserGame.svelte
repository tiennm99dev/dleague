<script context="module">

    /**
     * @typedef {Object} TPhaserRef
     * @property {import('phaser').Game | null} game
     * @property {import('phaser').Scene | null} scene
     */

</script>

<script>

    import { onMount } from "svelte";
    import StartGame from "./game/main";
    import { EventBus } from './game/EventBus';

    /** @type {TPhaserRef} */
    export let phaserRef = {
        game: null,
        scene: null
    };

    /** @type {(scene: import('phaser').Scene) => void | undefined} */
    export let currentActiveScene;

    onMount(() => {

        phaserRef.game = StartGame("game-container");

        EventBus.on('current-scene-ready', (/** @type {import('phaser').Scene} */ scene_instance) => {

            phaserRef.scene = scene_instance;

            if(currentActiveScene)
            {

                currentActiveScene(scene_instance);

            }

        });

    });

</script>

<div id="game-container"></div>