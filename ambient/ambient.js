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

  document.querySelector('#crickets-button').addEventListener('click', () => {
    resumeIfNecessary();
    sources.crickets.element.play();
  });

  // Port-forward to localhost for testing.
  //
  // kubectl port-forward `kubectl get pods -lapp=thanos-query -o jsonpath='{.items[0].metadata.name}'` 9090
  const loop = () => {
    setTimeout(async () => {
      try {
        const resp = await fetch(
          'http://localhost:9090/api/v1/query?query=sum%20(rate%20(container_cpu_usage_seconds_total%5B1m%5D))&dedup=true&partial_response=true',
          {
            headers: {
              accept: 'application/json',
            },
            body: null,
            method: 'GET',
            mode: 'cors',
          }
        );
        if (!resp.ok) {
          throw new Error('Failed to fetch');
        }
        const json = await resp.json();
        const value = +json.data.result[0].value[1];
        console.log(value);
        // Should be around 500, so set the gain of crickets appropriately.
        sources.crickets.gain.gain.value = (value - 500) / 100;
        console.log(sources.crickets.gain.gain.value);
      } catch (error) {
        console.log(error);
      } finally {
        loop();
      }
    }, 5000);
  };
  loop();
};

init();
