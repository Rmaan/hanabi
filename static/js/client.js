"use strict";

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
    document.body.innerHTML = `
<div id="top-bar">
	<div id="debug"></div>
	<button id='btn-dc'>DC</button>
</div>
<div id="canvas">
	<div id="status"></div>
    <div class="desk"></div>
    <div class="hanabis"></div>
    <div class="others-command-pallet hide">
        <button class="cmd-hint-color">Hint color (<span></span>)</button>
        <button class="cmd-hint-number">Hint number (<span></span>)</button>
    </div>
    <div class="player-0 players">
        <div class="cards"></div>
        <div class="name"></div>
        <div class="self-command-pallet hide">
            <button class="cmd-play">Play</button>
            <button class="cmd-discard">Discard</button>
        </div>
    </div>
    <div class="player-1 players"><div class="cards"></div><div class="name"></div></div>
    <div class="player-2 players"><div class="cards"></div><div class="name"></div></div>
    <div class="player-3 players"><div class="cards"></div><div class="name"></div></div>
    <div class="player-4 players"><div class="cards"></div><div class="name"></div></div>
</div>`

    var $canvas = document.getElementById('canvas');
    var $status = document.getElementById('status');
    var $debug = document.getElementById('debug');
    var $hanabis = document.querySelector('.hanabis');
    var $selfCmdPallet = document.querySelector('.self-command-pallet');
    var $othersCmdPallet = document.querySelector('.others-command-pallet');
    var hoveredSelfCardIndex = null
    var lastMoveLocationX, lastMoveLocationY
    var hoveredOthersCard = {
        active: false,
        playerId: null,
        card: null,
    }
    var $players = _.range(5).map(idx => document.querySelector('.player-' + idx))
    var $playerCards = $players.map(el => el.querySelector('.cards'))
    var $selfCards = $playerCards[0]
    var $othersCards = $playerCards.slice(1)

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

    function discardCard(cardIndex) {
        sendCommand('discard', {
            'CardIndex': cardIndex
        })
    }

    function playCard(cardIndex) {
        sendCommand('play', {
            'CardIndex': cardIndex
        })
    }

    function getChildNumber(node) {
        return Array.prototype.indexOf.call(node.parentNode.childNodes, node);
    }

    function hoverUnhoverOthersCard(active, playerId, $card) {
        // If active is false will unhover

        if (hoveredOthersCard.active == active && (!active || hoveredOthersCard.playerId == playerId && hoveredOthersCard.card == $card.extra.gameObj))
            return

        if (hoveredOthersCard.active) {

        }

        hoveredOthersCard.active = active

        $othersCmdPallet.classList.toggle('hide', !active)

        if (hoveredOthersCard.active) {
            hoveredOthersCard.playerId = playerId
            var card = hoveredOthersCard.card = $card.extra.gameObj
            var playerCardRect = $playerCards[playerId].getClientRects()[0];
            var canvasRect = $canvas.getClientRects()[0];
            var cardRect = $card.getClientRects()[0]

            if (playerId == 1 || playerId == 2) {
                $othersCmdPallet.style.left = playerCardRect.width + 'px'
                $othersCmdPallet.style.right = ''
                $othersCmdPallet.style.top = cardRect.y - canvasRect.y + 'px'
            }
            if (playerId == 3 || playerId == 4) {
                $othersCmdPallet.style.left = ''
                $othersCmdPallet.style.right = playerCardRect.width + 'px'
                $othersCmdPallet.style.top = cardRect.y - canvasRect.y + 'px'
            }

            $othersCmdPallet.querySelector('button:nth-child(1) span').textContent = card.Color
            $othersCmdPallet.querySelector('button:nth-child(2) span').textContent = card.Number
        }
    }

    $othersCards.forEach(($cards, idx) => $cards.onclick = e => {
        if (e.target.classList.contains('obj_player_card')) {
            hoverUnhoverOthersCard(true, idx + 1, e.target)
        }
    })

    document.body.addEventListener('click', e => {
        if (!e.target.classList.contains('obj_player_card')) {
            hoverUnhoverOthersCard(false)
        }
    })

    document.querySelector('.cmd-hint-color').onclick = e => hintPlayer(hoveredOthersCard.playerId, true, hoveredOthersCard.card.Color)
    document.querySelector('.cmd-hint-number').onclick = e => hintPlayer(hoveredOthersCard.playerId, false, hoveredOthersCard.card.Number)

    function hoverUnhoverSelfCard(index) {
        // Pass index to hover a specific card.
        // Pass null to unhover hovered card.
        // Can be called multiple times with same parameter without making a mess.
        if (hoveredSelfCardIndex === index)
            return

        if (hoveredSelfCardIndex !== null) {
            $selfCards.childNodes[hoveredSelfCardIndex].classList.remove('hover')
        }

        hoveredSelfCardIndex = index
        $selfCmdPallet.classList.toggle('hide', hoveredSelfCardIndex === null)

        if (hoveredSelfCardIndex !== null) {
            $selfCards.childNodes[hoveredSelfCardIndex].classList.add('hover')
        }
    }

    $selfCards.onclick = e => {
        if (e.target.classList.contains('obj_player_card')) {
            hoverUnhoverSelfCard(getChildNumber(e.target))
        }
    }

    document.querySelector('.cmd-play').onclick = e => {
        playCard(hoveredSelfCardIndex)
        hoverUnhoverSelfCard(null)
    }

    document.querySelector('.cmd-discard').onclick = e => {
        discardCard(hoveredSelfCardIndex)
        hoverUnhoverSelfCard(null)
    }

    document.body.addEventListener('click', e => {
        if (!e.target.classList.contains('obj_player_card')) {
            hoverUnhoverSelfCard(null)
        }
    })

    function getObjectDiv(obj, $parent) {
        var domId = 'game-id-' + obj.Id
        var $o = document.getElementById(domId)
        if (!$o) {
            $o = document.createElement('div')
            $o.id = domId
            $o.classList.add('obj_' + obj.Class, 'game-obj')
            $o.extra = {
                gameObj: obj,
                gameClass: obj.Class,
            }
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

            $parent.appendChild($o)
        }
        return $o
    }

    function drawWorld(world) {
        console.log('drawing', world)

        $status.textContent = `tick=${world.TickNumber} hint_token=${world.HintTokenCount} mistake_token=${world.MistakeTokenCount} deck_cards=${world.RemainingDeckCount}`;

        // TODO avoid creating divs each tick.
        $hanabis.innerHTML = ''
        world.SuccessfulPlayedCount.forEach((count, idx) => {
            var $hanabi = document.createElement('div')
            $hanabi.className = 'obj_desk_card'
            $hanabi.style.backgroundImage = `url("/static/img/spirits/${idx + 1}${count}.png")`
            $hanabis.appendChild($hanabi)
        })

        var allObjectsClass = {}
        document.querySelectorAll('.game-obj').forEach(el => {
            allObjectsClass[el.id] = el.extra.gameClass
        })

        // TODO support deleting div of removed objects
        // TODO remove Desk from client/server
        world.DeskObjects.forEach(obj => {
            if (obj.SpiritId) {
                obj.Class = 'desk_item'
            } else {
                obj.Class = 'block'
            }
            var $o = getObjectDiv(obj, $canvas)

            $o.style.top = obj.Y + 'px'
            $o.style.left = obj.X + 'px'

            if (obj.SpiritId) {
                $o.style.backgroundImage = `url("/static/img/spirits/${obj.SpiritId}.png")`
            }
        })

        world.Players.forEach((p, playerIndex) => {
            $players[playerIndex].querySelector('.name').textContent = p.Name

            p.Cards.forEach(obj => {
                obj.Class = 'player_card'
                var $o = getObjectDiv(obj, $playerCards[playerIndex])

                $o.style.backgroundImage = `url("/static/img/spirits/${obj.Color}${obj.Number}.png")`

                delete allObjectsClass[$o.id] // Remove them from dict so they remain in DOM
            })
        })

        // Remove all non-networked cards from DOM
        Object.keys(allObjectsClass).forEach(id => {
            if (allObjectsClass[id] == 'player_card') {
                document.getElementById(id).remove()
            }
        })
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
    window.hint = hintPlayer
    window.discard = discardCard
    window.play = playCard
});
