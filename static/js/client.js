(function() {
    // var codec = msgpack.createCodec();
    msgpack.codec.preset.addExtUnpacker(0x00, myVectorUnpacker);


    function myVectorUnpacker(buffer) {
        var array = msgpack.decode(buffer);
        var obj = {};
        var fields = ['Id', 'X', 'Y', 'Width', 'Height', 'Color', 'ColorHinted', 'Number', 'NumberHinted']
        array.forEach((x, idx) => obj[fields[idx]] = x)
        return obj
    }
})();

window.addEventListener("load", function() {
    var $canvas = document.getElementById('canvas');
    var $status = document.getElementById('status');
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

    document.getElementById('btn-dc').onclick = () => ws.close()

    function sendCommand(type, params) {
        var data = JSON.stringify({type: type, params: params})
        $debug.innerText = +new Date() + " " + data;
        ws.send(data)
    }

    var lastMoveLocationX, lastMoveLocationY
    var sendMoveCommand = _.throttle(ev => {
        // on drag over will be called multiple times event if you don't move your cursor.
        if (ev.clientX == lastMoveLocationX && ev.clientY == lastMoveLocationY)
            return
        lastMoveLocationX = ev.clientX
        lastMoveLocationY = ev.clientY

        var commandParams = {
            'TargetId': dragging.objectBeingDragged,
            'X': ev.clientX - $canvas.getClientRects()[0].x - dragging.gripOffsetX,
            'Y': ev.clientY - $canvas.getClientRects()[0].y - dragging.gripOffsetY,
        }
        sendCommand('move', commandParams)
    }, 50, {leading: false})

    $canvas.ondragover = ev => {
        ev.preventDefault()
        ev.dataTransfer.dropEffect = "move"
        sendMoveCommand(ev)
    }

    function flipItem(objId) {
        sendCommand('flip', {'TargetId': objId})
    }

    function hintPlayer(playerId, isColor, value) {
        sendCommand('hint', {
            'PlayerId': playerId,
            'IsColor': isColor,
            'Value': value,
        })
    }

    window.hint = hintPlayer

    function getObjectDiv(obj, scope) {
        var domId = 'game-id-' + obj.Id
        var $o = document.getElementById(domId)
        if (!$o) {
            $o = document.createElement('div')
            $o.id = domId
            $o.classList.add('obj_' + obj.Class, 'game-obj')
            $o.style.width = obj.Width + 'px'
            $o.style.height = obj.Height + 'px'

            if (obj.Class == 'desk_item') {
                $o.draggable = true
                $o.ondragstart = ev => {
                    dragging = {
                        objectBeingDragged: obj.Id,
                        gripOffsetX: ev.clientX - ev.target.getClientRects()[0].x,
                        gripOffsetY: ev.clientY - ev.target.getClientRects()[0].y,
                    }
                    console.log('drag start', dragging)
                }
                $o.onclick = ev => {
                    flipItem(obj.Id)
                }
            }

            $canvas.querySelector('.' + scope).appendChild($o)
        }
        return $o
    }

    function drawWorld(world) {
        console.log('drawing', world)

        $status.textContent = world.TickNumber;

        // TODO support deleting div of removed objects
        world.DeskObjects.forEach(obj => {
            if (obj.SpiritId) {
                obj.Class = 'desk_item'
            } else {
                obj.Class = 'block'
            }
            var $o = getObjectDiv(obj, 'desk')

            $o.style.top = obj.Y + 'px'
            $o.style.left = obj.X + 'px'

            if (obj.SpiritId) {
                $o.style.backgroundImage = 'url("/static/img/spirits/' + obj.SpiritId + '.png")'
            }
        })

        world.Players.forEach((p, playerIndex) => p.Cards.forEach(obj => {
            obj.Class = 'player_card'
            var $o = getObjectDiv(obj, 'player-' + playerIndex)

            $o.style.backgroundImage = 'url("/static/img/spirits/' + obj.Color + obj.Number + '.png")'
        }))
    }

    var ws = new WebSocket(window.args.ws_url);
    ws.binaryType = "arraybuffer"

    ws.onopen = function(e) {
        $status.textContent = "WS open"
    }

    ws.onclose = function(e) {
        $status.textContent = "WS closed :("
    }

    ws.onmessage = function(e) {
        var buffer = new Uint8Array(e.data)
        drawWorld(msgpack.decode(buffer))
    }

    ws.onerror = function(e) {
        console.log("WS err: " + e.data);
    }

    window.ws = ws
});
