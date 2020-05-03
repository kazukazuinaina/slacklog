[
    {
        "type": "message",
        "user": "U9GUS1S0G",
        "text": "すいません、お聞きしたいことがあります。\n`matchaddpos()` 関数が `TextChanged` イベントを発生させるのは仕様でしょうか？",
        "ts": "1521290789.000069"
    },
    {
        "type": "message",
        "user": "U9GUS1S0G",
        "text": "しかも、どうも最初の一回だけ発生させて二回目以降の関数呼び出しでは発生しないようなのです。",
        "ts": "1521290847.000015"
    },
    {
        "type": "message",
        "user": "U9GUS1S0G",
        "text": "```\naugroup Test\nautocmd!\nautocmd TextChanged * echomsg 'TextChanged'\naugroup END\n\ncall matchaddpos('IncSearch', [1])\n```\nこれを保存して `:source` すると最初の一回だけ `TextChanged` が表示されます",
        "ts": "1521290951.000016"
    },
    {
        "type": "message",
        "user": "U03C6TEAZ",
        "text": "一見バグっぽく見えますね。",
        "client_msg_id": "56c8d846-bf2e-4b41-bcc4-e0749badb54d",
        "ts": "1521333296.000025"
    },
    {
        "type": "message",
        "user": "U03C6TEAZ",
        "text": "んー。再現出来ないすね。何か条件ありますか？",
        "ts": "1521348727.000028"
    },
    {
        "type": "message",
        "user": "U9GUS1S0G",
        "text": "今のところ、 `vim -u NONE -N` で立ち上げて `:source %` すると100％再現しているんですが、何か見落としているかも\nもう少し調べて解決しなければ後で issue を建てます",
        "edited": {
            "user": "U9GUS1S0G",
            "ts": "1521353739.000000"
        },
        "ts": "1521353718.000048"
    }
]