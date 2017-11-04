window.addEventListener("load", function() {
    var $canvas = document.getElementById('canvas');
    var $tick = document.getElementById('tick');

    var x = 0
    function drawWorld(world) {
        if (world.TickNumber === undefined)
            return;
        console.log('drawing', world)
        x++

        $tick.textContent = world.TickNumber;
        $canvas.innerHTML = '';

        world.AllObjects.forEach(obj => {
            var $ch = document.createElement('div');
            $ch.className = 'block';
            $ch.style.top = obj.Y + 'px';
            $ch.style.left = obj.X + 'px';
            $ch.style.width = obj.Width + 'px';
            $ch.style.height = obj.Height + 'px';
            $canvas.appendChild($ch);
        });
    }

    var ws = new WebSocket(window.args.ws_url);
    ws.onopen = function(e) {
        console.log("WS open");
    }
    ws.onclose = function(e) {
        console.log("WS close");
    }
    ws.onmessage = function(e) {
        // console.log("RESPONSE: " + world);
        var reader = new FileReader()
        reader.onload = e => {
            var buffer = new Uint8Array(e.target.result)
            drawWorld(msgpack.decode(buffer))
        }
        reader.readAsArrayBuffer(e.data)
    }
    ws.onerror = function(e) {
        console.log("WS err: " + e.data);
    }

    window.ws = ws
});
