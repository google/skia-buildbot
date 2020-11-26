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
      // Create a gain node.
      const gainNode = audioCtx.createGain();
      gainNode.gain = 1;

      // Connect everything together and then to the output.
      src.connect(gainNode).connect(audioCtx.destination);
      sources[id] = {
        element: ele,
        gainNode: gainNode,
      };
    });
  });

  const resumeIfNecessary = () => {
    if (audioCtx.state === 'suspended') {
      audioCtx.resume();
    }
  };

  // cricket query
  const cricketFormula = 'sum (rate (container_cpu_usage_seconds_total[10m]))';

  const config = [
    {
      source: 'crickets',
      url: `http://localhost:9090/api/v1/query?query=${encodeURIComponent(
        cricketFormula
      )}`,
      extractValueFromResponse: (json) => +json.data.result[0].value[1],
      valueToGain: (value) => Math.log10(value) - 2,
    },

    {
      source: 'cicada',
      url: 'https://perf.skia.org/_/alerts/',
      extractValueFromResponse: (json) => json.alerts,
      valueToGain: (value) => (value > 0 ? 1 : 0),
    },
  ];

  let loopID = -1;
  // Port-forward to localhost for testing.
  //
  // kubectl port-forward `kubectl get pods -lapp=thanos-query -o jsonpath='{.items[0].metadata.name}'` 9090
  const loop = (timeout = 5000) => {
    setTimeout(() => {
      if (loopID === -1) {
        return;
      }
      config.forEach(async (cfg) => {
        try {
          const resp = await fetch(cfg.url, {
            headers: {
              accept: 'application/json',
            },
            body: null,
            method: 'GET',
            mode: 'cors',
          });
          if (!resp.ok) {
            throw new Error('Failed to fetch');
          }
          const json = await resp.json();
          const value = cfg.extractValueFromResponse(json);
          console.log(cfg.source, 'value', value);
          // Should be around 500, so set the gain of crickets appropriately.
          // Math.log10 maps 100 -> 2 and 1000 -> 3, so scale that down to a range of 0 to 1.
          const gain = cfg.valueToGain(value);
          sources[cfg.source].gainNode.gain.value = gain;
          if (gain > 0) {
            sources[cfg.source].element.play();
          }
          console.log(cfg.source, 'gain', gain);
        } catch (error) {
          console.log(cfg.source, error);
        }
      });
      loopID = loop();
    }, timeout);
  };

  document.querySelector('#alerts-button').addEventListener('click', () => {
    resumeIfNecessary();
    const button = document.getElementById('alerts-button');
    if (loopID === -1) {
      loopID = loop(1);
      button.innerText = 'Stop';
    } else {
      window.clearTimeout(loopID);
      loopID = -1;
      Object.keys(sources).forEach((id) => {
        sources[id].element.pause();
      });
      button.innerText = 'Start';
    }
  });
};

init();
