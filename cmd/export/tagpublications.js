// mongo script to add tags and publication ids to articles
// (should be done at source by the scrapomat eventually)
// eg:
// mongo localhost/scotland --quiet tagpublications.js



//db = db.getSiblingDB('eurobot')


// delete these
blacklist = [
	"video.ft.com",
	"subscriber.telegraph.co.uk",
	"s.telegraph.co.uk",
	"register.theguardian.com",
	"id.theguardian.com",
	"login.thetimes.co.uk",
	"registration.ft.com"
];

print("removing registration pages etc")
db.articles.remove({'publication.domain': {$in: blacklist}});


// add pub field and tag
augment = {
	"blogs.spectator.co.uk": {sn:'spectator', tag:'nat'},
	"blogs.telegraph.co.uk": {sn:'telegraph', tag:'nat'},
	"labourlist.org": {sn:'labourlist', tag:'blog'},
	"liberalconspiracy.org": {sn: 'liberalconspiracy', tag:'blog'},
	"order-order.com": {sn:  'order-order', tag:'blog'},
	"politicalscrapbook.net": {sn: 'politicalscrapbook', tag:'blog'},
	"www.politics.co.uk": {sn: 'politics.co.uk', tag:'blog'},
	"politicshome.com": {sn: 'politicshome', tag:'blog'},
	"ukpollingreport.co.uk": {sn: 'ukpollingreport', tag:'blog'},
	"www.bbc.co.uk": {sn: 'bbc', tag:'nat'},
	"www.express.co.uk": {sn: 'express', tag:'nat'},
	"www.conservativehome.com": {sn: 'conservativehome', tag:'blog'},
	"www.dailymail.co.uk": {sn:'dailymail', tag:'nat'},
	"www.ft.com": {sn: 'ft', tag:'nat'},
	"www.leftfootforward.org": {sn:'leftfootforward', tag:'blog'},
	"www.iaindale.com": {sn: 'iaindale', tag:'blog'},
	"www.independent.co.uk": {sn: 'independent', tag:'nat'},
	"live.independent.co.uk": {sn: 'independent', tag:'nat'},
	"blogs.independent.co.uk": {sn: 'independent', tag:'nat'},
	"www.mirror.co.uk": {sn: 'mirror', tag:'nat'},
	"www.irishmirror.ie": {sn: 'mirror', tag:'nat'},
	"www.dailyrecord.co.uk": {sn: 'dailyrecord', tag:'nat'},
	"www.newstatesman.com": {sn: 'newstatesman', tag:'nat'},
	"www.spectator.co.uk": {sn: 'spectator', tag:'nat'},
	"www.telegraph.co.uk": {sn: 'telegraph', tag:'nat'},
	"fashion.telegraph.co.uk": {sn: 'telegraph', tag:'nat'},
	"www.thecommentator.com": {sn: 'thecommentator', tag:'blog'},
	"www.theguardian.com": {sn: 'guardian', tag:'nat'},
	"www.thesun.co.uk": {sn: 'sun', tag:'nat'},
	"www.thesundaytimes.co.uk": {sn: 'sundaytimes', tag:'nat'},
	"www.thetimes.co.uk": {sn: 'times', tag:'nat'},
	"www.scotsman.com": {sn: 'scotsman', tag:'scot'},
	"www.dailyrecord.co.uk": {sn: 'dailyrecord', tag:'scot'},
	"www.heraldscotland.com": {sn: 'herald', tag:'scot'},
	"www.eveningtimes.co.uk": {sn: 'eveningtimes', tag:'scot'},
	"www.eveningtelegraph.co.uk": {sn: 'thetele', tag:'scot'}
};

for (var domain in augment) {
    var shortname = augment[domain].sn;
    var tag = augment[domain].tag;

    db.articles.update(
        { 'publication.domain':domain },
        { $set: { 'pub':shortname }, $addToSet: {'tags':tag} },
        { multi: true } );
    print(domain + ": shortname '" + shortname + "', tag '" + tag + "'");
    //print(shortname + ": " + db.articles.find({'pub':shortname}).count() + " articles");
}





