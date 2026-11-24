package main

func init() {
	onlineCuratedBank = append(onlineCuratedBank, idiomCuratedBank...)
}

// idiomCuratedBank 按五大年龄学段（3-5岁启蒙 / 6-7岁一年级 / 8岁二年级 / 9-10岁三四年级 / 11-14岁高年级）分难度梯度的成语精选库（共 120 题）。
var idiomCuratedBank = []QuizQuestion{
	// ==================== 3-5 岁 · 启蒙趣味成语（数字/动物/生活画面） (+24题) ====================
	// easy (+8)
	{ID: "idm_35_e1", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "启蒙成语填空：五（  ）六色，形容颜色特别多、特别好看。", Options: []string{"颜", "朵", "只"}, AnswerIdx: 0},
	{ID: "idm_35_e2", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "形容春天到来、花儿都开了的成语是哪一个？", Options: []string{"春暖花开", "冰天雪地", "落叶飘零"}, AnswerIdx: 0},
	{ID: "idm_35_e3", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "启蒙成语填空：一（  ）一意，表示做事情很认真专心。", Options: []string{"心", "头", "脚"}, AnswerIdx: 0},
	{ID: "idm_35_e4", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "小朋友收到喜欢的礼物、开开心心地笑，可以用哪个成语来形容？", Options: []string{"欢天喜地", "垂头丧气", "大发雷霆"}, AnswerIdx: 0},
	{ID: "idm_35_e5", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "启蒙成语填空：七（  ）八下，形容心里紧张或者慌慌张张的。", Options: []string{"上", "左", "前"}, AnswerIdx: 0},
	{ID: "idm_35_e6", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "公园里小鸟唱歌、花儿散发香味，可以用哪个成语形容？", Options: []string{"鸟语花香", "鸦雀无声", "电闪雷鸣"}, AnswerIdx: 0},
	{ID: "idm_35_e7", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "做事一会儿想玩玩具、一会儿想看动画片，不专心叫什么成语？", Options: []string{"三心二意", "专心致志", "全神贯注"}, AnswerIdx: 0},
	{ID: "idm_35_e8", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "启蒙成语填空：十（  ）十美，表示做得特别好、没有缺点。", Options: []string{"全", "半", "少"}, AnswerIdx: 0},

	// normal (+8)
	{ID: "idm_35_n1", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "写作业或搭积木时，眼睛总是一会儿看左边、一会儿看右边不专心，叫作？", Options: []string{"东张西望", "目不转睛", "埋头苦干"}, AnswerIdx: 0},
	{ID: "idm_35_n2", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "哪一个成语里藏着小动物“兔”，讲的是农夫不干活只想捡便宜的故事？", Options: []string{"守株待兔", "马到成功", "鱼目混珠"}, AnswerIdx: 0},
	{ID: "idm_35_n3", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "小青蛙一直住在深井里，以为天空只有井口那么大，这个成语故事叫？", Options: []string{"井底之蛙", "如鱼得水", "顺手牵羊"}, AnswerIdx: 0},
	{ID: "idm_35_n4", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "启蒙成语填空：四面（  ）方，指周围的各个方向。", Options: []string{"八", "两", "零"}, AnswerIdx: 0},
	{ID: "idm_35_n5", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "在故事《对牛弹琴》里，音乐家对着牛弹优美的琴声，结果怎么样？", Options: []string{"牛听不懂，继续低头吃草", "牛跟着跳起舞来", "牛学会了弹琴"}, AnswerIdx: 0},
	{ID: "idm_35_n6", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "形容大马路上汽车来来往往、非常热闹的成语是？", Options: []string{"车水马龙", "门可罗雀", "风平浪静"}, AnswerIdx: 0},
	{ID: "idm_35_n7", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "做事情只做了一半就放弃不干了，是用下面哪个成语形容？", Options: []string{"半途而废", "坚持不懈", "大功告成"}, AnswerIdx: 0},
	{ID: "idm_35_n8", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "启蒙成语填空：百（  ）百中，形容射箭或投篮非常准。", Options: []string{"发", "飞", "跑"}, AnswerIdx: 0},

	// hard (+8)
	{ID: "idm_35_h1", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "小朋友讲的故事《狐假虎威》里，小狐狸是靠谁的威风吓跑森林里其他动物的？", Options: []string{"大老虎", "小兔子", "大灰狼"}, AnswerIdx: 0},
	{ID: "idm_35_h2", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "《盲人摸象》中，几个摸大象的叔叔只摸到大象的一小部分就乱猜，这个成语告诉我们？", Options: []string{"看事物要全面，不能只看一部分", "大象像一根粗柱子", "大象和墙一模一样"}, AnswerIdx: 0},
	{ID: "idm_35_h3", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "小禾苗需要慢慢长高，有个人等不及就把禾苗一棵棵往上拔，结果禾苗全枯死了。这叫？", Options: []string{"拔苗助长", "春风化雨", "绿树成荫"}, AnswerIdx: 0},
	{ID: "idm_35_h4", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "启蒙成语填空：九牛一（  ），比喻数量非常多里的一点点。", Options: []string{"毛", "角", "尾"}, AnswerIdx: 0},
	{ID: "idm_35_h5", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "一个人说话前后自己和自己打架，既说“盾最坚固”又说“矛最锋利”，这叫作？", Options: []string{"自相矛盾", "实话实说", "言行一致"}, AnswerIdx: 0},
	{ID: "idm_35_h6", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "坐船时宝剑掉进水里，有人不马上捞，却只在移动的船边刻个记号，这个愚蠢的做法叫？", Options: []string{"刻舟求剑", "按图索骥", "水落石出"}, AnswerIdx: 0},
	{ID: "idm_35_h7", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "以下哪个成语中包含两种不同的小动物名称？", Options: []string{"狐假虎威", "鸟语花香", "万马奔腾"}, AnswerIdx: 0},
	{ID: "idm_35_h8", Category: "语文", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "成语“千方百计”里面一共有两个表示数字的汉字，请问它们分别是？", Options: []string{"千和百", "方和计", "十和万"}, AnswerIdx: 0},

	// ==================== 6-7 岁 · 一年级基础成语（常用积累/结构成语） (+24题) ====================
	// easy (+8)
	{ID: "idm_67_e1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "做一件事同时得到了两个好结果，可以用下面哪个成语表示？", Options: []string{"一举两得", "徒劳无功", "得不偿失"}, AnswerIdx: 0},
	{ID: "idm_67_e2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "一年级成语填空：千山万（  ），形容路途遥远，经过很多高山和大河。", Options: []string{"水", "树", "花"}, AnswerIdx: 0},
	{ID: "idm_67_e3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“手舞足蹈”这个成语通常用来描写人处于什么心情？", Options: []string{"非常开心、高兴", "非常害怕、发抖", "特别伤心、难过"}, AnswerIdx: 0},
	{ID: "idm_67_e4", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "一个人自己跟自己小声说话，可以用哪个 ABAC 式成语形容？", Options: []string{"自言自语", "自由自在", "无忧无虑"}, AnswerIdx: 0},
	{ID: "idm_67_e5", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "形容走路步子迈得特别大、走得很快的成语是？", Options: []string{"大步流星", "小心翼翼", "慢条斯理"}, AnswerIdx: 0},
	{ID: "idm_67_e6", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "一年级成语填空：五光（  ）色，形容色彩鲜艳、花样繁多。", Options: []string{"十", "七", "万"}, AnswerIdx: 0},
	{ID: "idm_67_e7", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "形容春天花园里各种花儿争相开放的热闹景象，用哪个成语最恰当？", Options: []string{"百花齐放", "硕果累累", "寒风凛冽"}, AnswerIdx: 0},
	{ID: "idm_67_e8", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "下列成语中，表示心情特别高兴、精神饱满的是？", Options: []string{"兴高采烈", "愁眉苦脸", "无精打采"}, AnswerIdx: 0},

	// normal (+8)
	{ID: "idm_67_n1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "看图画书或者吃美味点心时觉得特别有趣、很有味道，可以用哪个 AABC 式成语？", Options: []string{"津津有味", "默默无闻", "闷闷不乐"}, AnswerIdx: 0},
	{ID: "idm_67_n2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "形容大海或大草原非常辽阔，看不到边界的成语是？", Options: []string{"无边无际", "井井有条", "微不足道"}, AnswerIdx: 0},
	{ID: "idm_67_n3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "大家抢着往前走，生怕自己落后了，这个情景叫作？", Options: []string{"争先恐后", "不紧不慢", "袖手旁观"}, AnswerIdx: 0},
	{ID: "idm_67_n4", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "一年级成语填空：山（  ）水秀，形容风景非常优美。", Options: []string{"清", "高", "青"}, AnswerIdx: 0},
	{ID: "idm_67_n5", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "把一件本来很普通的小事看得特别了不起、非常吃惊，这叫作？", Options: []string{"大惊小怪", "镇定自若", "平心静气"}, AnswerIdx: 0},
	{ID: "idm_67_n6", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "游乐园里游客非常多、密密麻麻像山像海一样，通常用什么成语形容？", Options: []string{"人山人海", "空无一人", "冷冷清清"}, AnswerIdx: 0},
	{ID: "idm_67_n7", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "下面哪个成语是由两对反义词（“左”对“右”，“顾”对“盼”表示看）组成、形容向四周张望？", Options: []string{"左顾右盼", "一心一意", "千言万语"}, AnswerIdx: 0},
	{ID: "idm_67_n8", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "同学们你一言我一语，对刚刚听到的故事热烈讨论，这叫什么成语？", Options: []string{"议论纷纷", "鸦雀无声", "心照不宣"}, AnswerIdx: 0},

	// hard (+8)
	{ID: "idm_67_h1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "在别人最困难、最需要帮助的时候伸出援手，这个温暖的成语叫？", Options: []string{"雪中送炭", "落井下石", "袖手旁观"}, AnswerIdx: 0},
	{ID: "idm_67_h2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "与“雪中送炭”相对，本来已经很好了还要加上更美的点缀，叫作？", Options: []string{"锦上添花", "多此一举", "杯水车薪"}, AnswerIdx: 0},
	{ID: "idm_67_h3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "成语故事《愚公移山》主要赞扬了愚公什么样的可贵精神？", Options: []string{"不怕困难、坚持不懈的毅力", "力气大能搬石头", "不喜欢住在大山里"}, AnswerIdx: 0},
	{ID: "idm_67_h4", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "叶公家里到处画着龙，但真龙飞来时他却吓跑了。成语“叶公好龙”比喻什么样的人？", Options: []string{"嘴上说喜欢某事物，其实并非真喜欢", "最擅长画龙的画家", "特别勇敢的捉龙人"}, AnswerIdx: 0},
	{ID: "idm_67_h5", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "想去南边的楚国却一直向北边赶马车，行动和目标完全相反，这个成语是？", Options: []string{"南辕北辙", "轻车熟路", "一帆风顺"}, AnswerIdx: 0},
	{ID: "idm_67_h6", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "一年级成语填空：千（  ）一发，形容事情到了极其危险紧要的关头。", Options: []string{"钧", "斤", "万"}, AnswerIdx: 0},
	{ID: "idm_67_h7", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "小鸟每天叼来树枝和石子想要填平大海，体现坚持不懈决心的是哪个神话成语？", Options: []string{"精卫填海", "狐假虎威", "画蛇添足"}, AnswerIdx: 0},
	{ID: "idm_67_h8", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "“熟能生巧”这个成语告诉我们什么学习道理？", Options: []string{"只要勤加练习，做事情就会越来越熟练灵巧", "睡个好觉就能变聪明", "不需要练习也能学会"}, AnswerIdx: 0},

	// ==================== 8 岁 · 二年级常用成语（寓言道理/四季描写/品质词汇） (+24题) ====================
	// easy (+8)
	{ID: "idm_8_e1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "二年级成语填空：名列前（  ），形容学习或比赛成绩排在最前面。", Options: []string{"茅", "矛", "猫"}, AnswerIdx: 0},
	{ID: "idm_8_e2", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "描写冬天天气非常寒冷、大地都结了冰的成语是？", Options: []string{"天寒地冻", "春色满园", "烈日当空"}, AnswerIdx: 0},
	{ID: "idm_8_e3", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "形容大风停了、海面很平静的成语是哪一个？", Options: []string{"风平浪静", "惊涛骇浪", "狂风暴雨"}, AnswerIdx: 0},
	{ID: "idm_8_e4", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "二年级成语填空：绿树成（  ），形容夏天树木茂盛、树下很凉爽。", Options: []string{"荫", "音", "阴"}, AnswerIdx: 0},
	{ID: "idm_8_e5", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "下列成语中，用来形容天气特别暖和、阳光灿烂、微风轻拂的是？", Options: []string{"风和日丽", "鹅毛大雪", "乌云密布"}, AnswerIdx: 0},
	{ID: "idm_8_e6", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "“井井有条”这个成语通常用来夸奖小朋友做什么做得好？", Options: []string{"把物品或事情整理得很有秩序、不乱", "会打井水", "会画长长的线条"}, AnswerIdx: 0},
	{ID: "idm_8_e7", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "二年级成语填空：自（  ）自足，形容靠自己的劳动就能满足生活需要。", Options: []string{"给", "己", "记"}, AnswerIdx: 0},
	{ID: "idm_8_e8", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "形容教室里非常安静，连乌鸦麻雀的声音都听不到，用哪个成语？", Options: []string{"鸦雀无声", "人声鼎沸", "熙熙攘攘"}, AnswerIdx: 0},

	// normal (+8)
	{ID: "idm_8_n1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "形容心里急切得连一秒钟都等不下去，可以用哪个成语？", Options: []string{"迫不及待", "从容不迫", "慢条斯理"}, AnswerIdx: 0},
	{ID: "idm_8_n2", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "北宋画家文同画竹子前早就想好竹子的样子了。成语“胸有成竹”比喻？", Options: []string{"做事之前心里早就有了把握和打算", "胸口上画了一根竹子", "特别喜欢吃竹笋"}, AnswerIdx: 0},
	{ID: "idm_8_n3", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "二年级成语填空：水滴石（  ），告诉我们只要有恒心不断努力，再难的事也能做成。", Options: []string{"穿", "破", "碎"}, AnswerIdx: 0},
	{ID: "idm_8_n4", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "受过箭伤的大雁听到弓弦一响就吓得掉下来，“惊弓之鸟”比喻？", Options: []string{"受过惊吓、一遇到一点动静就非常害怕的人", "飞得特别高的大雁", "射箭技术很好"}, AnswerIdx: 0},
	{ID: "idm_8_n5", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "张僧繇画龙最后点上眼睛，龙就破壁飞走了。“画龙点睛”常比喻说话或写文章？", Options: []string{"在关键处用一两句精彩的话使内容更有神采", "随便多画几笔多余的画", "只画眼睛不画身体"}, AnswerIdx: 0},
	{ID: "idm_8_n6", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "二年级成语填空：锲而不（  ），比喻雕刻一件东西一直坚持不放弃。", Options: []string{"舍", "弃", "停"}, AnswerIdx: 0},
	{ID: "idm_8_n7", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "形容经过很多次失败后总结经验，最后还是取得成功，下面哪个成语与它意思接近？", Options: []string{"大器晚成", "一蹶不振", "半途而废"}, AnswerIdx: 0},
	{ID: "idm_8_n8", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "“和颜悦色”这个成语描写的是人的什么方面？", Options: []string{"态度温和、脸色和蔼可亲", "穿的衣服颜色鲜艳", "画画颜料很多"}, AnswerIdx: 0},

	// hard (+8)
	{ID: "idm_8_h1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "战国时期廉颇背着荆条向蔺相如登门道歉，这个知错就改的成语故事叫？", Options: []string{"负荆请罪", "完璧归赵", "三顾茅庐"}, AnswerIdx: 0},
	{ID: "idm_8_h2", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "越王勾践战败后睡在柴草上、尝苦胆激励自己发奋图强，这个成语是？", Options: []string{"卧薪尝胆", "闻鸡起舞", "悬梁刺股"}, AnswerIdx: 0},
	{ID: "idm_8_h3", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "把墙上的弓倒影当成了杯子里的毒蛇，自己吓自己导致生病，这个成语叫？", Options: []string{"杯弓蛇影", "草木皆兵", "惊弓之鸟"}, AnswerIdx: 0},
	{ID: "idm_8_h4", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "行军途中士兵口渴，曹操说前面有梅林酸梅解渴，大家想起梅子就流口水。这叫？", Options: []string{"望梅止渴", "画饼充饥", "对牛弹琴"}, AnswerIdx: 0},
	{ID: "idm_8_h5", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "晋朝祖逖一听到半夜鸡叫就起床练剑读书，这个勤奋励志的成语叫作？", Options: []string{"闻鸡起舞", "鸡鸣狗盗", "呆若木鸡"}, AnswerIdx: 0},
	{ID: "idm_8_h6", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "“唇亡齿寒”用来比喻两个事物或国家之间怎样的关系？", Options: []string{"关系非常密切，互相依存，共患难", "牙齿怕冷", "互相敌对竞争"}, AnswerIdx: 0},
	{ID: "idm_8_h7", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "蔺相如机智勇敢地把和氏璧完好无损带回赵国，这个成语叫？", Options: []string{"完璧归赵", "价值连城", "纸上谈兵"}, AnswerIdx: 0},
	{ID: "idm_8_h8", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "“黔驴技穷”里的那头驴一开始吓坏了老虎，后来本领全部暴露后结果怎样？", Options: []string{"被老虎识破并吃掉，比喻有限的本领用完了", "打败了森林里的老虎", "和老虎成了好朋友"}, AnswerIdx: 0},

	// ==================== 9-10 岁 · 三~四年级成语（历史典故/品质修养/词义辨析） (+24题) ====================
	// easy (+8)
	{ID: "idm_910_e1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "三四年级成语填空：精（  ）求精，指已经很好了还追求更加完美。", Options: []string{"益", "意", "艺"}, AnswerIdx: 0},
	{ID: "idm_910_e2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "形容大江大河水势浩大、波浪起伏非常壮观，通常用哪一个成语？", Options: []string{"波澜壮阔", "涓涓细流", "风和日丽"}, AnswerIdx: 0},
	{ID: "idm_910_e3", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "捡到别人遗失的钱包或贵重物品主动归还失主，这种高尚品质叫？", Options: []string{"拾金不昧", "贪小便宜", "据为己有"}, AnswerIdx: 0},
	{ID: "idm_910_e4", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "三四年级成语填空：络（  ）不绝，形容行人、车马或游客前后接连不断。", Options: []string{"绎", "译", "驿"}, AnswerIdx: 0},
	{ID: "idm_910_e5", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "形容春天草木萌发、充满活力和生命力的成语是？", Options: []string{"生机勃勃", "死气沉沉", "枯木逢春"}, AnswerIdx: 0},
	{ID: "idm_910_e6", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "岳飞母亲在他背上刺字希望他尽忠报答国家，对应下面哪个成语？", Options: []string{"精忠报国", "忠言逆耳", "投笔从戎"}, AnswerIdx: 0},
	{ID: "idm_910_e7", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "下面哪个成语与“全神贯注”意思最相近，都表示注意力高度集中？", Options: []string{"专心致志", "心不在焉", "魂不守舍"}, AnswerIdx: 0},
	{ID: "idm_910_e8", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "形容说话流利不停、非常动听，像河水顺流而下一样，叫什么成语？", Options: []string{"滔滔不绝", "吞吞吐吐", "哑口无言"}, AnswerIdx: 0},

	// normal (+8)
	{ID: "idm_910_n1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "刘备三次亲自去茅屋拜访并诚心邀请诸葛亮出山相助，这个故事叫？", Options: []string{"三顾茅庐", "桃园结义", "草船借箭"}, AnswerIdx: 0},
	{ID: "idm_910_n2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "杨时和游酢站在门外大雪中等候老师程颐醒来，成语“程门立雪”主要赞扬什么精神？", Options: []string{"尊师重道、虚心求学", "喜欢冬天在雪地里玩", "身体很强壮不怕冷"}, AnswerIdx: 0},
	{ID: "idm_910_n3", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "西汉匡衡把墙壁凿个小洞借邻居家灯光夜读，东晋车胤捉萤火虫照明读书，这两个成语分别是？", Options: []string{"凿壁偷光与囊萤映雪", "悬梁刺股与闻鸡起舞", "韦编三绝与手不释卷"}, AnswerIdx: 0},
	{ID: "idm_910_n4", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "项羽渡河后下令凿沉船只、砸碎做饭的锅，表示必胜决心，这个成语叫？", Options: []string{"破釜沉舟", "背水一战", "孤注一掷"}, AnswerIdx: 0},
	{ID: "idm_910_n5", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "孔子晚年反复研读《周易》，把穿竹简的牛皮绳都磨断了好几次，这叫？", Options: []string{"韦编三绝", "洛阳纸贵", "汗牛充栋"}, AnswerIdx: 0},
	{ID: "idm_910_n6", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "成语“塞翁失马”出自《淮南子》，它的后半句俗语常说“焉知非福”，表达了什么哲理？", Options: []string{"坏事在一定条件下也可能变成好事", "丢失马匹永远找不回来了", "边塞的人都不喜欢养马"}, AnswerIdx: 0},
	{ID: "idm_910_n7", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "齐桓公迷路时依靠老马带路走出荒野，“老马识途”后来多用来比喻？", Options: []string{"阅历丰富、熟悉情况的人能起到引导作用", "只有马匹才能认出回家的路", "年轻人不应该做向导"}, AnswerIdx: 0},
	{ID: "idm_910_n8", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "三四年级成语辨析填空：再接再（  ），指继续努力、不松懈（注意不要写错字）。", Options: []string{"厉", "励", "历"}, AnswerIdx: 0},

	// hard (+8)
	{ID: "idm_910_h1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "淝水之战中，前秦苻坚把远山上的草木都看成了东晋士兵，极度惊慌。这个成语是？", Options: []string{"草木皆兵", "兵临城下", "调虎离山"}, AnswerIdx: 0},
	{ID: "idm_910_h2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "与“草木皆兵”同出自淝水之战，形容听到风声和鹤叫都以为敌军追来、惊恐万状的是？", Options: []string{"风声鹤唳", "风起云涌", "鹤立鸡群"}, AnswerIdx: 0},
	{ID: "idm_910_h3", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "秦二世时赵高当朝把鹿说成马试探大臣，成语“指鹿为马”用来比喻什么行为？", Options: []string{"故意颠倒黑白、混淆是非", "对动物分类不熟悉", "眼神不好看错了动物"}, AnswerIdx: 0},
	{ID: "idm_910_h4", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "战国时期赵国食客毛遂主动站出来请求出使楚国并立功，“毛遂自荐”现在常指？", Options: []string{"自己主动推荐自己承担某项任务", "谦虚地把机会让给别人", "被朋友强行拉去帮忙"}, AnswerIdx: 0},
	{ID: "idm_910_h5", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "春秋时期晋文公遵守诺言，主动让军队后撤九十里来感谢楚国当年的照顾，“退避三舍”中一舍是多少里？", Options: []string{"三十里（三舍共九十里）", "十里（三舍共三十里）", "一百里（三舍共三百里）"}, AnswerIdx: 0},
	{ID: "idm_910_h6", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "霸王项羽被刘邦军队包围，夜里听到四面营帐外都唱起楚国歌谣，“四面楚歌”形容？", Options: []string{"陷入四面受敌、孤立无援的困境", "军队里大家都非常喜欢唱歌", "战场上举行音乐比赛"}, AnswerIdx: 0},
	{ID: "idm_910_h7", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "东晋谢安隐居东山多年后再次出任高官建功立业，“东山再起”通常比喻？", Options: []string{"失败或隐退后重新恢复声势与地位", "每天早上在东边山上散步", "去东边重新建造一座山"}, AnswerIdx: 0},
	{ID: "idm_910_h8", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "下面哪个成语的主人公不是三国时期的人物？", Options: []string{"四面楚歌（项羽）", "三顾茅庐（刘备、诸葛亮）", "望梅止渴（曹操）"}, AnswerIdx: 0},

	// ==================== 11-14 岁 · 高年级/初中高频易错与典故成语 (+24题) ====================
	// easy (+8)
	{ID: "idm_1114_e1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "《论语》有云：“敏而好学，不耻下问。”其中“不耻下问”的意思是？", Options: []string{"不以向地位或学问不如自己的人请教为耻辱", "只要遇到不懂的问题就只问老师", "耻笑下面同学问出的简单问题"}, AnswerIdx: 0},
	{ID: "idm_1114_e2", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "高年级成语填空：循序（  ）进，意为按一定顺序和步骤逐步深入提高。", Options: []string{"渐", "剑", "建"}, AnswerIdx: 0},
	{ID: "idm_1114_e3", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "“见贤思齐”这句话激励青少年见到德才兼备的人时应该怎么做？", Options: []string{"向他看齐、学习他的优点", "嫉妒并疏远他", "觉得自己绝对追不上"}, AnswerIdx: 0},
	{ID: "idm_1114_e4", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "出自《论语》形容担子很重、路途遥远，常用来比喻责任重大且需要长期奋斗的成语是？", Options: []string{"任重道远", "举重若轻", "轻而易举"}, AnswerIdx: 0},
	{ID: "idm_1114_e5", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "形容学习或工作特别专心努力，甚至连睡觉和吃饭都忘记了，用哪个成语？", Options: []string{"废寝忘食", "寝食难安", "饱食终日"}, AnswerIdx: 0},
	{ID: "idm_1114_e6", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "高年级成语填空：实事求（  ），意为从客观实际出发去探求事物规律。", Options: []string{"是", "事", "实"}, AnswerIdx: 0},
	{ID: "idm_1114_e7", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "成语“博学笃志”中，“笃志”表达的意思是？", Options: []string{"意志坚定专一", "志向非常短暂", "只读医学书籍"}, AnswerIdx: 0},
	{ID: "idm_1114_e8", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "比喻学习或积累知识需要一点一滴慢慢来，不能急于求成，下面哪项符合这个意思？", Options: []string{"厚积薄发", "一步登天", "急功近利"}, AnswerIdx: 0},

	// normal (+8)
	{ID: "idm_1114_n1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "出自《列子》，比喻为不必要或毫无根据的事情忧愁害怕的成语是？", Options: []string{"杞人忧天", "居安思危", "未雨绸缪"}, AnswerIdx: 0},
	{ID: "idm_1114_n2", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "爬到树上去找鱼（《孟子》“缘木求鱼”），比喻什么？", Options: []string{"方向或方法不对，不可能达到目的", "树上经常能抓到鱼", "鱼儿喜欢爬树晒太阳"}, AnswerIdx: 0},
	{ID: "idm_1114_n3", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "在鲁班门前舞弄斧子，“班门弄斧”用来比喻什么？", Options: []string{"在行家面前卖弄本领，不自量力", "木匠技术非常好", "向古代大师致敬"}, AnswerIdx: 0},
	{ID: "idm_1114_n4", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "高考与中考常考易错成语：“差强人意”原本准确的词义是什么？", Options: []string{"勉强使人还算满意（中性略偏褒义）", "让人非常不满意、太差了", "强迫别人同意自己的意见"}, AnswerIdx: 0},
	{ID: "idm_1114_n5", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "常考易错成语辨析：“首当其冲”中的“冲”指交通要冲，这个成语正确的意思是？", Options: []string{"比喻最先受到攻击、冲击或遭遇灾难", "第一个勇敢向前冲锋", "每次比赛都是第一名"}, AnswerIdx: 0},
	{ID: "idm_1114_n6", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "春秋伯牙鼓琴、钟子期听音，“高山流水”这个典故常用来比喻？", Options: []string{"知音难觅或乐曲高妙", "山上的泉水特别凉快", "爬山渡水十分艰难"}, AnswerIdx: 0},
	{ID: "idm_1114_n7", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "“曲高和寡”这个成语最初讲的道理是？", Options: []string{"言论或艺术作品越深奥高雅，能理解和附和的人越少", "唱歌声音越高越容易走音", "曲子太长大家不想听"}, AnswerIdx: 0},
	{ID: "idm_1114_n8", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "常考易错成语：“炙手可热”比喻权势很大、气焰很盛使人不敢接近，通常带有什么感情色彩？", Options: []string{"贬义（多指权贵气焰盛，不宜用来夸天气热或东西抢手）", "纯褒义（常用来夸奖明星很受欢迎）", "只能用来描写烧烤食物"}, AnswerIdx: 0},

	// hard (+8)
	{ID: "idm_1114_h1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "高年级易错成语辨析：“美轮美奂”原意出自《礼记》，它在现代汉语中通常只能用来形容？", Options: []string{"房屋建筑高大华美，或装饰布置富丽堂皇（不能形容风景/人）", "自然山川的秀丽景色", "人物外貌俊美或歌声动听"}, AnswerIdx: 0},
	{ID: "idm_1114_h2", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "下列句子中，成语“巧夺天工”使用最恰当的一项是？（提示：巧夺天工指人工精巧胜过天然）", Options: []string{"博物馆里这件微雕玉器雕工精细，堪称巧夺天工", "黄山奇松怪石是大自然的杰作，真是巧夺天工", "这片天然形成的大峡谷令人惊叹，巧夺天工"}, AnswerIdx: 0},
	{ID: "idm_1114_h3", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "柳宗元《陆文通先生墓表》提到运书能让牛累出汗、堆满屋梁，“汗牛充栋”常用来形容？", Options: []string{"藏书或著作数量极多", "水牛在夏天辛勤耕地", "房屋建筑材料十分厚重"}, AnswerIdx: 0},
	{ID: "idm_1114_h4", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "出自《庄子》“泉涸，鱼相与处于陆……相濡以沫”，这个成语比喻？", Options: []string{"在困境中用微薄力量互相救助、共渡难关", "小鱼喜欢晒太阳吐泡泡", "在游泳池里互相泼水嬉戏"}, AnswerIdx: 0},
	{ID: "idm_1114_h5", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "“韬光养晦”里的“韬”是隐藏，“晦”是暗淡不显露，这个成语常指？", Options: []string{"隐藏才能、不急于外露，暗中积蓄力量", "天天在黑暗房间里睡觉", "大张旗鼓炫耀自己的才华"}, AnswerIdx: 0},
	{ID: "idm_1114_h6", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "常考词义辨析：“空穴来风”最初的传统本义是“有洞穴才会有风吹进来”，用来比喻？", Options: []string{"消息和传说的产生往往有一定根据（现代也常误作毫无根据，考试重本义）", "洞穴里一定住着妖怪", "风都是从墙缝里吹出来的"}, AnswerIdx: 0},
	{ID: "idm_1114_h7", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "“罄竹难书”中的“罄”是尽、“竹”指古代竹简，这个成语用来形容？", Options: []string{"罪恶或坏事多得写不完（严禁用于形容好人好事）", "功劳和好事多得写不完", "竹简太轻不好写字"}, AnswerIdx: 0},
	{ID: "idm_1114_h8", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "高年级语法辨析：下列对成语“莘莘学子”的使用完全正确的一项是？", Options: []string{"金秋九月，来自全国各地的莘莘学子齐聚校园", "一位莘莘学子正坐在图书馆窗边专心读书", "这三名莘莘学子一起获得了科技创新奖"}, AnswerIdx: 0},
}
