var fs = require('fs');
var obj = JSON.parse(fs.readFileSync('./trustpilot_reviews_smarteducationpro.co.uk.json', 'utf8'));

console.log(obj);
