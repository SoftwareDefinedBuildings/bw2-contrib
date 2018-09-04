## Pelican Thermostat Driver
### Setting up a New Site
Before running the Pelican driver at a new site, you must first configure the
site's thermostats. This is done with a standalone tool available in the
`discovery` directory.

The discovery tool first uses the Pelican API to discover the thermostats that
are installed at the relevant site and then stores information about these
thermostats in a remote database for fault-tolerance purposes. It requires the
presence of four files in the current working directory when it is executed:
  1. `params.yml`: This is identical in structure to the parameter file for the
     driver itself. The file must contain the following YAML key/value pairs:
    1. `username`: The username to access the Pelican API
    2. `password`: The password to access the Pelican API
    3. `sitename`: The name of the site, as it is referred to by Pelican
  2. A CA certificate file (e.g. `ca.crt`)
  3. A certificate file for the remote database (e.g. `my_db.crt`)
  4. A private key file for authentication with the database (e.g. `my_db.key`)

### Running the Driver
Once information about the site's thermostats has been stored in the database,
the Pelican driver is ready to run.

When executed, the driver first reads thermostat information from the database
before entering the typical Bosswave publish/subscribe loop. Because it
interacts with the same database as the discovery tool, the driver requires the
presence of the *same four files* in the current working directory when it is
executed. Note, however, that the driver expects additional key/value pairs to
be present in the `params.yml` file.
