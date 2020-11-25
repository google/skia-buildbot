const init = () => {
  const audioCtx = new window.AudioContext();

  const sourceNames = [
    'blackbird',
    'cicada',
    'crickets',
    'frogs',
    'loon',
    'mockingbird',
    'rain',
    'thunder',
    'wind',
  ];

  const sources = {};

  function setFadeout(element, gain) {
    gain.gain.setValueAtTime(1.0, audioCtx.currentTime);
    gain.gain.linearRampToValueAtTime(
      0,
      audioCtx.currentTime + element.duration - 5.0
    );
  }

  sourceNames.forEach((id) => {
    const ele = document.getElementById(id);
    const src = audioCtx.createMediaElementSource(ele);
    ele.addEventListener('loadedmetadata', () => {
      // Connect it though the lowpass filter.
      /*
      const biquadFilter = audioCtx.createBiquadFilter();
      biquadFilter.type = 'lowpass';
      biquadFilter.frequency.value = 200;
      */

      // And then add some gain.
      // Create a gain node for some sounds.
      const gainNode = audioCtx.createGain();
      gainNode.gain = 2;

      // Finally connect to the output.
      src.connect(gainNode).connect(audioCtx.destination);
      sources[id] = {
        element: ele,
        gain: gainNode,
      };
    });
  });

  const resumeIfNecessary = () => {
    if (audioCtx.state === 'suspended') {
      audioCtx.resume();
    }
  };

  document.querySelector('#blackbird-button').addEventListener('click', () => {
    resumeIfNecessary();
    sources.blackbird.element.play();
  });

  document.querySelector('#thunder-button').addEventListener('click', () => {
    resumeIfNecessary();
    setFadeout(sources.thunder.element, sources.thunder.gain);
    sources.thunder.element.play();
  });
};

init();
