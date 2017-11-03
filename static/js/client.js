window.addEventListener("load", function(evt) {
    var $canvas = document.getElementById('canvas');
    var $tick = document.getElementById('tick');

    var x = 0
    function drawWorld(world) {
        if (world.TickNumber === undefined)
            return;
        x++

        $tick.textContent = world.TickNumber;
        $canvas.innerHTML = '';

        world.AllObjects.forEach(obj => {
            var $ch = document.createElement('div');
            $ch.className = 'block';
            $ch.style.top = obj.Y + 'px';
            $ch.style.left = obj.X + 'px';
            $canvas.appendChild($ch);
        });
    }

    var ws = new WebSocket(window.args.ws_url);
    window.ws = ws;
    ws.onopen = function(evt) {
        console.log("OPEN");
    }
    ws.onclose = function(evt) {
        console.log("CLOSE");
    }
    ws.onmessage = function(evt) {
        console.log("RESPONSE: " + evt.data);
        drawWorld(JSON.parse(evt.data));
    }
    ws.onerror = function(evt) {
        console.log("ERROR: " + evt.data);
    }
    //ws.close();
});
