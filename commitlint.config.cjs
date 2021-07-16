module.exports = {
    rules: {
        // Ensure we have a blank line between the header and the body.
       'body-leading-blank': [2, 'always'],
       // Ensure the body ends with a period.
       'body-full-stop': [2, 'always'],
       // Ensure we don't have lines longer than 72 chars.
       'body-max-line-length': [2, 'always', 72],
       // Ensure the header line doesn't end with a period.
       'header-full-stop': [2, 'never'],
    },
};
