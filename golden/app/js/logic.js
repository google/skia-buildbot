'use strict';


/* Add this to the skia namespace */
var skia = skia || {};
(function (ns) {

    /**
     * Plot is a class that wraps the flot object and exposes draw functions.
     *
     * @param {jQueryElement} element This a jquery element to which the
                                      flot instance to attach to.
     *
     * @return {Plot} instance of Plot class.
     **/

	ns.Plot = function (element) {
		this.element = element;

		// initialize the flot element with empty data.
		this.flotObj = element.plot([], {
			legend: {
				show: true
			}
	    }).data('plot');
	};


	/**
	 * setData sets the data that the plot needs to draw and forces a redraw.
	 * If ticks is not null it will also set the ticks and reset the x-axis.
	 *
     * @param {array} data Array of series understood by Flot. See
     *                     https://github.com/flot/flot/blob/master/API.md#data-format
     *
     * @param {array} ticks Array of flot ticks to be placed on x-axis.
	*/
	ns.Plot.prototype.setData = function(data, ticks) {
		this.flotObj.setData(data);

		// Set the ticks if necessary
		if (ticks) {
			this.flotObj.getOptions().xaxis.ticks = ticks;
		}

		// redraw the graph
		this.flotObj.setupGrid();
		this.flotObj.draw();
	};

	/**
	 * PlotData is a class that used as the return value of processAllCounts and
	 * contains the processed data.
	 *	*/
	ns.PlotData = function (data, ticks, allAggregates, testDetails) {
		this.plotData = data;
		this.plotTicks = ticks;
		this.testDetails = testDetails;
		this.allAggregates = allAggregates;
	};


	/**
	* TestDetails is a class that contails the aggregated information about
	* a single tests. It is derived from the data returned by the server.
	*/
	ns.TestDetails = function (name, counts) {
		this.name = name;
		this.counts = counts;
		this.aggregates = aggregateCounts(counts);
	};

	/**
	* addIndexAsX adds takes an array of numbers and returns an array of
	* datapoints (x,y) where x is the index of the input element y.
	*/
	function addIndexAsX(arr) {
		var result = [];
		for(var i=0, len=arr.length; i<len; i++) {
			result.push([i, arr[i]])
		}
		return result;
	}

	/**
	* aggregateCounts sums over the counts contained in an object.
	* Each member in the object is assumed to be an array of numbers.
	*
	* @param { object } countsObj contains attributes where each attribute
	*                   is an array of numbers.
	* @return {object} an array with the same attributes as the input object.
	*                  Each attribute contains the sum of the corresponding
	*                  array.
	*/
	function aggregateCounts(countsObj) {
		var result = {};
		var arr;
		for(var k in countsObj) {
			if (countsObj.hasOwnProperty(k)) {
				result[k] = 0;
				arr = countsObj[k];
				for(var i=0,len=arr.length; i < len; i++) {
					result[k] += arr[i];
				}
			}
		}
		return result;
	}

	/**
	* processAllCounts converts the data returned by the server to
	*  an instance of PlotData that can then be used to render the UI
	*  and also serve as input to the Plot class.
	*
	* @param {object} serverData returned from the server containing the
	*                 aggregated values over all tests.
	*
	* @return {object} instance of PlotData.
	*
	*/
	ns.processAllCounts = function (serverData) {
		// get the counts from the tests.
		var testCounts = [];
		for(var testName in serverData.counts) {
			if (serverData.counts.hasOwnProperty(testName)) {
				testCounts.push(new ns.TestDetails(testName, serverData.counts[testName]));
			}
		}

		// add the ticks.
		var ticks = [-0.5, serverData.commits.length+1];

		// assemble the plot data.
		var data = [];
		for(var k in serverData.aggregated) {
			if (serverData.aggregated.hasOwnProperty(k)) {
				data.push({
					label: k,
					lines: {
						show: true,
						steps: true
					},
					data: addIndexAsX(serverData.aggregated[k])
				});
			}
		}


		return new ns.PlotData(data, ticks, aggregateCounts(serverData.aggregated), testCounts);
	};

})(skia);
