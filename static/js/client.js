window.addEventListener("load", function() {
    var $canvas = document.getElementById('canvas');
    var $tick = document.getElementById('tick');
    var $debug = document.getElementById('debug');
    // Browsers doesn't support passing data to ondragover (for a reason I don't know) this is a simple workaround
    // assuming there is only one drag (no multi touch).
    var dragging = {
        objectBeingDragged: null,
        gripOffsetX: 0,
        gripOffsetY: 0,
    }

    $canvas.ondrop = ev => {
        // ev.preventDefault()
        console.log('drop', ev,  ev.dataTransfer.getData("text"))
    }

    // TODO throttle these events
    $canvas.ondragover = ev => {
        ev.preventDefault()
        ev.dataTransfer.dropEffect = "move"
        console.log('over', ev)
        var commandParams = {
            'TargetId': dragging.objectBeingDragged,
            'X': ev.clientX - $canvas.getClientRects()[0].x - dragging.gripOffsetX,
            'Y': ev.clientY - $canvas.getClientRects()[0].y - dragging.gripOffsetY,
        }
        var data = JSON.stringify({type: 'move', params: commandParams})
        $debug.innerText = data;
        ws.send(data)
    }

    var x = 0
    function drawWorld(world) {
        if (world.TickNumber === undefined)
            return;
        // console.log('drawing', world)
        x++

        $tick.textContent = world.TickNumber;
        $canvas.innerHTML = '';

        world.AllObjects.forEach(obj => {
            var $ch = document.createElement('div')
            $ch.className = 'block'
            $ch.style.top = obj.Y + 'px'
            $ch.style.left = obj.X + 'px'
            $ch.style.width = obj.Width + 'px'
            $ch.style.height = obj.Height + 'px'
            if (obj.SpiritId) {
                $ch.classList.add('spirit')
                $ch.style.backgroundImage = 'url(/static/img/cards/' + obj.SpiritId + '.png)'
                $ch.draggable = true
                $ch.ondragstart = ev => {
                    dragging = {
                        objectBeingDragged: obj.Id,
                        gripOffsetX: ev.clientX - ev.target.getClientRects()[0].x,
                        gripOffsetY: ev.clientY - ev.target.getClientRects()[0].y,
                    }
                    console.log('drag start', dragging)
                }
                $ch.ondragend = ev => {
                    dragging.objectBeingDragged = null
                }
            } else {
                $ch.classList.add('no-spirit')
            }
            $canvas.appendChild($ch)
        })
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
