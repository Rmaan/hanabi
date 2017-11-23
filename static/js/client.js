"use strict";

(function() {
    // var codec = msgpack.createCodec();
    msgpack.codec.preset.addExtUnpacker(0x00, myVectorUnpacker);


    function myVectorUnpacker(buffer) {
        var array = msgpack.decode(buffer);
        var obj = {};
        var fields = ['Id', 'Color', 'ColorHinted', 'Number', 'NumberHinted']
        array.forEach((x, idx) => obj[fields[idx]] = x)
        return obj
    }
})();

window.addEventListener("load", function() {
    var h = maquette.h;
    var projector = maquette.createProjector();

    var status = 'Hello!';
    var debug = 'debug bar';
    // World networked from server
    var w = {
        SuccessfulPlayedCount: [],
        Players: [],
        DeskObjects: [],
    }
    var allMsgs = []  // All chat logs
    var hoveredSelfCardIndex = null
    var hoveredOthersCard = {
        active: false,
        playerId: null,
        card: null,
        $card: null,
    }
    var isRenaming = false
    var colorNames = [
        "UNKNOWN COLOR",
        "Purple",
        "Sky Blue",
        "Orange",
        "Magenta",
        "Green",
    ]
    var colorValues = [
        "#dddddd",
        "#6850a1",
        "#40b9ff",
        "#ef6f36",
        "#e70e72",
        "#1f8e22",
    ]

    // Do first render
    projector.append(document.body, render);

    var $canvas = document.getElementById('canvas');
    var $msgBox = document.getElementById('msg-box');
    var $msgLog = $msgBox.querySelector('.msg-log');

    function renderHanabis() {
        return h('div.hanabis',
            w.SuccessfulPlayedCount.map((count, idx) =>
                count > 0 ?
                    h('div.obj_desk_card', {key: idx, styles: {'background-image': `url("/static/img/spirits/${idx + 1}${count}.png")`}}) :
                    h('div.obj_desk_card', {key: idx + 100, styles: {'backgroundColor': colorValues[idx + 1], 'opacity': '0.3'}})
            )
        )
    }

    function renderMsgBox() {
        return h('div#msg-box', [
            h('div.msg-log', allMsgs.map((log, idx) =>
                h('div.line', {key: idx, classes: {"system-msg": !log.IsChat}}, [
                    h('span.player-name', [w.Players[log.PlayerId].Name]),
                    ': ',
                    h('span.text', [log.Text])
                ])
            )),
            h('div.send', [
                h('form', {
                    onsubmit: onSubmitChat,
                }, [
                    h('input', {type: 'text'}),
                    h('button', {type: 'submit'}, ['Send']),
                ])
            ])
        ])
    }

    function renderPlayer(playerId) {
        var player = w.Players[playerId]
        if (!player)
            return null

        var isSelf = playerId == 0

        var selfPallet = isSelf ? h('div.self-command-pallet.hide', {
            classes: {hide: hoveredSelfCardIndex == null}
        }, [
            h('button.cmd-play', {onclick: onCmdPlay}, ['Play']),
            h('button.cmd-discard', {onclick: onCmdDiscard}, ['Discard']),
        ]) : null

        return h(`div.players.player-${playerId}`, [
            h('div.cards', player.Cards.map((card, idx) =>
                h('div.obj_player_card', {
                    styles: {backgroundImage: `url("/static/img/spirits/${card.Color}${card.Number}.png")`},
                    classes: {hover: isSelf && idx == hoveredSelfCardIndex},
                    key: card.id,
                    onclick: isSelf ? (e => hoveredSelfCardIndex = idx) : (e => setOthersCardHoverState(true, playerId, card, e.target))
                })
            )),
            h('div.name', [
                isSelf && isRenaming ?
                    h('form', {
                        onsubmit: onSubmitRename
                    }, [
                        h('input', {value: player.Name})
                    ])
                :
                    h('span', {
                        onclick: isSelf && !isRenaming ? (e => isRenaming = true) : null
                    }, [
                        player.Name
                    ])
            ]),
            selfPallet
        ])
    }

    function renderOthersCmdPallet() {
        if (!hoveredOthersCard.active)
            return null

        // Want to place command pallet near the card clicked. We extract some coordinates using DOM directly.
        var playerCardRect = document.querySelector(`.player-${hoveredOthersCard.playerId} .cards`).getClientRects()[0]
        var canvasRect = $canvas.getClientRects()[0]
        var cardRect = hoveredOthersCard.$card.getClientRects()[0]

        var styles = {
            top: cardRect.y - canvasRect.y + 'px',
            right: '',
            left: '',
        }

        if (hoveredOthersCard.playerId == 1 || hoveredOthersCard.playerId == 2) {
            styles.left = playerCardRect.width + 'px'
        } else if (hoveredOthersCard.playerId == 3 || hoveredOthersCard.playerId == 4) {
            styles.right = playerCardRect.width + 'px'
        }

        return h('div.others-command-pallet', {
            styles: styles,
        }, [
            h('button.cmd-hint-color', {onclick: onCmdHintColor}, [`Hint ${colorNames[hoveredOthersCard.card.Color]}`]),
            h('button.cmd-hint-number', {onclick: onCmdHintNumber}, [`Hint ${hoveredOthersCard.card.Number}`]),
        ])
    }

    function renderDesk() {
        return h('div.desk', w.DeskObjects.map(obj =>
            h('div.obj_block', {
                styles: {
                    top: obj.Y + 'px',
                    left: obj.X + 'px',
                    width: obj.Width.toString(),
                    height: obj.Height.toString(),
                },
            })
        ))

    }

    function render() {
        return h('div.render', [
            h('div.top-bar', [
                h('div#debug', [debug]),
                h('button#btn-dc', {onclick: e => ws.close()}, ['DC'])
            ]),
            h('div#canvas', [
                h('div#status', [status]),
                renderDesk(),
                renderHanabis(),
                renderOthersCmdPallet(),
                renderPlayer(0),
                renderPlayer(1),
                renderPlayer(2),
                renderPlayer(3),
                renderPlayer(4),
                renderMsgBox(),
            ]),
        ]);
    }

    function sendCommand(type, params) {
        var data = JSON.stringify({type: type, params: params})
        debug = +new Date() + " " + data;
        ws.send(data)
        projector.scheduleRender()
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

    function setOthersCardHoverState(active, playerId, card, $card) {
        hoveredOthersCard.active = active
        if (active) {
            hoveredOthersCard.playerId = playerId
            hoveredOthersCard.card = card
            hoveredOthersCard.$card = $card
        } else {
            hoveredOthersCard.playerId = null
            hoveredOthersCard.card = null
            hoveredOthersCard.$card = null
        }
    }

    function onCmdHintColor(e) {
        hintPlayer(hoveredOthersCard.playerId, true, hoveredOthersCard.card.Color)
    }

    function onCmdHintNumber(e) {
        hintPlayer(hoveredOthersCard.playerId, false, hoveredOthersCard.card.Number)
    }

    function onCmdPlay(e) {
        playCard(hoveredSelfCardIndex)
        hoveredSelfCardIndex = null
        projector.scheduleRender()
    }

    function onCmdDiscard(e) {
        discardCard(hoveredSelfCardIndex)
        hoveredSelfCardIndex = null
        projector.scheduleRender()
    }

    function onSubmitRename(e) {
        e.preventDefault()
        var $inp = e.target.querySelector('input')
        sendCommand('rename', {
            NewName: $inp.value
        })
        isRenaming = false
    }

    // Hide pallets when clicked on somewhere else
    document.body.addEventListener('click', e => {
        if (!e.target.classList.contains('obj_player_card')) {
            // unhover self
            hoveredSelfCardIndex = null

            // unhover others
            setOthersCardHoverState(false)

            projector.scheduleRender()
        }

        if (!e.target.matches('.player-0 > .name *')) {
            // cancel renaming
            isRenaming = false

            projector.scheduleRender()
        }
    })

    function onSubmitChat(e) {
        e.preventDefault()
        var $inp = e.target.querySelector('input')
        sendCommand('chat', {
            Text: $inp.value
        })
        $inp.value = ''
    }

    function setNewWorld(world) {
        w = world;
        window.w = w;
        status = `tick=${world.TickNumber} hint_token=${world.HintTokenCount} mistake_token=${world.MistakeTokenCount} deck_cards=${world.RemainingDeckCount}`;
        allMsgs = allMsgs.concat(world.NewLogs)

        // Auto scroll down msg log
        if (world.NewLogs.length)
            $msgLog.scrollTop = $msgLog.scrollHeight

        projector.scheduleRender()
    }

    var ws = new WebSocket(window.args.ws_url);
    ws.binaryType = "arraybuffer"

    ws.onopen = e => {
        status = "WS open"
        projector.scheduleRender()
    }

    ws.onclose = e => {
        status = "WS closed :("
        projector.scheduleRender()
    }

    ws.onmessage = e => {
        var buffer = new Uint8Array(e.data)
        setNewWorld(msgpack.decode(buffer))
    }

    ws.onerror = e => {
        console.log("WS err: " + e.data);
    }

    window.cmd = sendCommand
});
